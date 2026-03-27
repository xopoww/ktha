package controller

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/xopoww/ktha/node/internal/config"
)

type InvalidImageError struct {
	ImagePath string
	Err       error
}

func (e InvalidImageError) Unwrap() error {
	return e.Err
}

func (e InvalidImageError) Error() string {
	return fmt.Sprintf("invalid image %q: %s", e.ImagePath, e.Err)
}

func validateImage(imagePath string) error {
	info, err := os.Stat(imagePath)
	if err != nil {
		return InvalidImageError{
			ImagePath: imagePath,
			Err:       fmt.Errorf("stat: %w", err),
		}
	}
	if !info.IsDir() {
		return InvalidImageError{
			ImagePath: imagePath,
			Err:       errors.New("image is not a directory"),
		}
	}
	return nil
}

var ErrInvalidEnv = errors.New("invalid env")

func validateEnv(env config.AppEnv) error {
	for key, val := range env {
		if key == "" {
			return fmt.Errorf("%w: empty key", ErrInvalidEnv)
		}
		if strings.ContainsAny(key, ",=") {
			return fmt.Errorf("%w: invalid key %q", ErrInvalidEnv, key)
		}
		if strings.Contains(val, ",") {
			return fmt.Errorf("%w: invalid value %q", ErrInvalidEnv, val)
		}
	}
	return nil
}
