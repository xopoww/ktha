package controller

import (
	"errors"
	"fmt"
	"os"
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
