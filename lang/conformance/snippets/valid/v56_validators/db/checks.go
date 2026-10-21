package db

import "errors"

// SiteKnown is the Go-reference check target (§validators): one parameter
// per argument column, spelled as the generated type, returning error.
func SiteKnown(site string) error {
	if site == "" {
		return errors.New("unknown site")
	}
	return nil
}
