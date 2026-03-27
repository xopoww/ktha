package manager

import (
	"errors"
)

var (
	ErrAppNotFound         = errors.New("app not found")
	ErrManagerShuttingDown = errors.New("manager is shutting down")
)
