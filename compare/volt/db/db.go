// Package db is the blog's data package: schema.volt declares the
// tables and selects.volt the queries; this file holds the validators
// the tables name (§validators).
package db

import (
	"errors"
	"regexp"
	"unicode/utf8"
)

var (
	slugShape = regexp.MustCompile(`^[a-z0-9-]{1,60}$`)
	tagShape  = regexp.MustCompile(`^[a-z0-9-]{1,20}$`)
)

// SlugShape refuses a slug outside [a-z0-9-]{1,60}.
func SlugShape(slug string) error {
	if !slugShape.MatchString(slug) {
		return errors.New("must match [a-z0-9-]{1,60}")
	}
	return nil
}

// TitleLength refuses a title over 200 characters.
func TitleLength(title string) error {
	if utf8.RuneCountInString(title) > 200 {
		return errors.New("must be at most 200 characters")
	}
	return nil
}

// TagShape refuses a tag outside [a-z0-9-]{1,20}.
func TagShape(tag string) error {
	if !tagShape.MatchString(tag) {
		return errors.New("must match [a-z0-9-]{1,20}")
	}
	return nil
}

// CommentLength refuses a comment over 1000 characters.
func CommentLength(body string) error {
	if utf8.RuneCountInString(body) > 1000 {
		return errors.New("must be at most 1000 characters")
	}
	return nil
}
