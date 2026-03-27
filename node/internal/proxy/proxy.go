package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/xopoww/ktha/node/internal/container"
	"github.com/xopoww/ktha/node/internal/manager"
	"go.uber.org/zap"
)

var appIdContextKey struct{}

func NewReverseProxy(mgr *manager.AppManager, l *zap.SugaredLogger) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			inPath := r.In.URL.Path
			appID, outPath := parseInPath(inPath)
			r.Out.URL.Path = outPath
			r.Out.URL.Scheme = "http"
			r.Out.URL.Host = appID
			r.Out = r.Out.WithContext(
				context.WithValue(r.Out.Context(), appIdContextKey, appID),
			)
			l.Debugf("Incoming request %q -> [%s]%q.", inPath, appID, outPath)
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				appID, ok := ctx.Value(appIdContextKey).(string)
				if !ok {
					return nil, fmt.Errorf("missing appID in context")
				}
				return mgr.DialApp(ctx, appID)
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
}

// parseInPath parses `/<appID>/out/path` into `appID` and `/out/path`
func parseInPath(inPath string) (appID string, outPath string) {
	inPath = strings.TrimPrefix(inPath, "/")
	appID, rest, _ := strings.Cut(inPath, "/")
	return appID, "/" + rest
}
