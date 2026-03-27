package admin

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/xopoww/ktha/node/internal/config"
	"github.com/xopoww/ktha/node/internal/controller"
	"github.com/xopoww/ktha/node/internal/manager"
)

func upgradeAppHandler(deps AdminDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		err := deps.Mgr.UpgradeApp(req.AppID, req.NewImage)
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
}

func addAppHandler(deps AdminDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			AppID string        `json:"appId"`
			Image string        `json:"image"`
			Env   config.AppEnv `json:"env"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		if req.AppID == "" || req.Image == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err := deps.Mgr.AddApp(manager.AppSpec{
			ID:    req.AppID,
			Image: req.Image,
			Env:   req.Env,
		})
		if errors.Is(err, manager.ErrAppAlreadyExists) {
			w.WriteHeader(http.StatusConflict)
		} else if errors.Is(err, manager.ErrManagerShuttingDown) {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if iiErr, ok := errors.AsType[*controller.InvalidImageError](err); ok {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(iiErr.Error()))
		} else if errors.Is(err, controller.ErrInvalidEnv) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(err.Error()))
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Write([]byte("{\"ok\": true}"))
		}
	})
}

func deleteAppHandler(deps AdminDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			AppID string `json:"appId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		if req.AppID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err := deps.Mgr.DeleteApp(req.AppID)
		if errors.Is(err, manager.ErrAppNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else if errors.Is(err, manager.ErrManagerShuttingDown) {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Write([]byte("{\"ok\": true}"))
		}
	})
}
