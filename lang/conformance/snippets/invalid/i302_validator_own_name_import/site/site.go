package site

import "errors"

// TitleShape refuses an empty title (§validators).
func TitleShape(title string) error {
	if title == "" {
		return errors.New("empty")
	}
	return nil
}
