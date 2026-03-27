package admin

import (
	"crypto/subtle"
	"fmt"
	"net/http"

	"github.com/xopoww/ktha/node/internal/config"
	"github.com/xopoww/ktha/node/internal/manager"
	"github.com/xopoww/ktha/node/internal/metrics"
	"go.uber.org/zap"
)

type AdminDeps struct {
	Cfg config.AdminConfig
	Mgr *manager.AppManager
	L   *zap.SugaredLogger
}

func NewAdminServer(deps AdminDeps) *http.Server {
	mux := http.NewServeMux()

	mux.Handle("/metrics", metrics.Handler())
	mux.Handle("/apps/upgrade", upgradeAppHandler(deps))
	mux.Handle("/apps/add", addAppHandler(deps))
	mux.Handle("/apps/delete", deleteAppHandler(deps))

	return &http.Server{
		Addr: fmt.Sprintf(":%d", deps.Cfg.Port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("x-api-key")
			if subtle.ConstantTimeCompare([]byte(key), []byte(deps.Cfg.AuthKey)) != 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			deps.L.Debugf("Admin request: %s %s.", r.Method, r.URL.Path)
			mux.ServeHTTP(w, r)
		}),
	}
}
