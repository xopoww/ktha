package admin

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/xopoww/ktha/node/internal/config"
	"github.com/xopoww/ktha/node/internal/controller"
	"github.com/xopoww/ktha/node/internal/manager"
	"go.uber.org/zap"
)

func NewAdminServer(cfg config.AdminConfig, mgr *manager.AppManager, l *zap.SugaredLogger) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/apps/upgrade", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			AppID    string `json:"appId"`
			NewImage string `json:"newImage"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		if req.AppID == "" || req.NewImage == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err := mgr.UpgradeApp(req.AppID, req.NewImage)
		if errors.Is(err, manager.ErrAppNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else if errors.Is(err, manager.ErrManagerShuttingDown) {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if iiErr, ok := errors.AsType[*controller.InvalidImageError](err); ok {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(iiErr.Error()))
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Write([]byte("{\"ok\": true}"))
		}
	})

	return &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("x-api-key")
			if subtle.ConstantTimeCompare([]byte(key), []byte(cfg.AuthKey)) != 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			l.Debugf("Admin request: %s %s.", r.Method, r.URL.Path)
			mux.ServeHTTP(w, r)
		}),
	}
}
