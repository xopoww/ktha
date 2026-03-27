package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/xopoww/ktha/node/internal/container"
	"github.com/xopoww/ktha/node/internal/manager"
	"github.com/xopoww/ktha/node/internal/metrics"
	"go.uber.org/zap"
)

var contextKey struct{}

type requestContext struct {
	appID     string
	coldStart bool
	dialOk    bool
}

func NewReverseProxy(mgr *manager.AppManager, l *zap.SugaredLogger) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			inPath := r.In.URL.Path
			appID, outPath := parseInPath(inPath)
			r.Out.URL.Path = outPath
			r.Out.URL.Scheme = "http"
			r.Out.URL.Host = appID

			rc, ok := r.In.Context().Value(contextKey).(*requestContext)
			if !ok {
				return
			}
			rc.appID = appID

			l.Debugf("Incoming request %q -> [%s]%q.", inPath, appID, outPath)
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				rc, ok := ctx.Value(contextKey).(*requestContext)
				if !ok {
					return nil, fmt.Errorf("missing value in context")
				}
				conn, coldStart, err := mgr.DialApp(ctx, rc.appID)
				rc.coldStart = coldStart
				rc.dialOk = err == nil
				return conn, err
			},
			// important: keepalive would bypass idle timeour timer reset
			DisableKeepAlives: true,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			l.Debugf("Proxy error: %s.", err)
			if errors.Is(err, manager.ErrAppNotFound) {
				w.WriteHeader(http.StatusNotFound)
			} else if errors.Is(err, container.ErrContainerDown) {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else if errors.Is(err, manager.ErrManagerShuttingDown) {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusBadGateway)
			}

		},
	}

	// wrap for metrics collection
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := &requestContext{}
		r = r.WithContext(context.WithValue(r.Context(), contextKey, rc))

		start := time.Now()
		proxy.ServeHTTP(w, r)
		duration := time.Since(start)

		metrics.ProxyRequestDuration.WithLabelValues(rc.appID, fmt.Sprint(rc.coldStart), fmt.Sprint(rc.dialOk)).Observe(duration.Seconds())
	})
}

// parseInPath parses `/<appID>/out/path` into `appID` and `/out/path`
func parseInPath(inPath string) (appID string, outPath string) {
	inPath = strings.TrimPrefix(inPath, "/")
	appID, rest, _ := strings.Cut(inPath, "/")
	return appID, "/" + rest
}
