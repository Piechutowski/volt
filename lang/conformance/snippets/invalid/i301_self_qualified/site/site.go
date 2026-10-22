package site

import (
	"errors"
	"net/http"

	"github.com/Piechutowski/volt"
)

// TitleShape refuses an empty title (§validators).
func TitleShape(title string) error {
	if title == "" {
		return errors.New("empty")
	}
	return nil
}

// Session is a plug (§pipelines).
func Session(next http.Handler) http.Handler { return next }

// Errors is the error handler (§scopes).
func Errors(w http.ResponseWriter, r *volt.Request, err error) {}
