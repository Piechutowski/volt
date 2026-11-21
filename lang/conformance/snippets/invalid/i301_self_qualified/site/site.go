package site

import (
	"errors"
	"net/http"

	"github.com/Piechutowski/volt"
)

// TitleShape refuses an empty title (spec/validators).
func TitleShape(title string) error {
	if title == "" {
		return errors.New("empty")
	}
	return nil
}

// Session is a plug (spec/pipelines).
func Session(next http.Handler) http.Handler { return next }

// Errors is the error handler (spec/scopes).
func Errors(r volt.Request, err error) volt.Response { return volt.DefaultErrorHandler(r, err) }
