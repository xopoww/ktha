package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/xopoww/ktha/node/internal/apps"
	"go.uber.org/zap"
)

var appIdContextKey struct{}

func NewReverseProxy(mgr *apps.AppManager, log *zap.Logger) *httputil.ReverseProxy {
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
			log.Sugar().Debugf("Incoming request %q -> [%s]%q.", inPath, appID, outPath)
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				appID, ok := ctx.Value(appIdContextKey).(string)
				if !ok {
					return nil, fmt.Errorf("missing appID in context")
				}
				if appID == "" {
					return nil, apps.ErrAppNotFound
				}

				socket, err := mgr.DialApp(ctx, appID)
				if err != nil {
					return nil, fmt.Errorf("dial app: %w", err)
				}
				log.Sugar().Debugf("Dial %q: %s.", appID, socket)

				return net.Dial("unix", socket)
			},
			// important: keepalive would bypass idle timeour timer reset
			DisableKeepAlives: true,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Sugar().Debugf("Proxy error: %s.", err)
			if errors.Is(err, apps.ErrAppNotFound) {
				w.WriteHeader(http.StatusNotFound)
			} else if errors.Is(err, apps.ErrAppIsDead) {
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
