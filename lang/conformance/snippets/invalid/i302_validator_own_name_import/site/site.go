package site

import "errors"

// TitleShape refuses an empty title (spec/validators).
func TitleShape(title string) error {
	if title == "" {
		return errors.New("empty")
	}
	return nil
}
