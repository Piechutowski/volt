package server

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/Piechutowski/volt/compare/chisqlc/api"
)

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9-]{1,60}$`)
	tagPattern  = regexp.MustCompile(`^[a-z0-9-]{1,20}$`)
)

const (
	maxTags          = 5
	maxTitle         = 200
	minPublishedBody = 100
	maxComment       = 1000
)

func validateNewPost(in api.NewPost) map[string]string {
	errs := map[string]string{}
	if !slugPattern.MatchString(in.Slug) {
		errs["slug"] = "must match [a-z0-9-]{1,60}"
	}
	validateTitle(errs, in.Title)
	if len(in.Tags) > maxTags {
		errs["tags"] = fmt.Sprintf("at most %d tags", maxTags)
	} else {
		for _, t := range in.Tags {
			if !tagPattern.MatchString(t) {
				errs["tags"] = fmt.Sprintf("tag %q must match [a-z0-9-]{1,20}", t)
				break
			}
		}
	}
	return errs
}

func validatePostEdit(in api.PostEdit) map[string]string {
	errs := map[string]string{}
	if in.Title != nil {
		validateTitle(errs, *in.Title)
	}
	return errs
}

func validateTitle(errs map[string]string, title string) {
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitle {
		errs["title"] = fmt.Sprintf("must be 1 to %d characters", maxTitle)
	}
}

func validateNewComment(in api.NewComment) map[string]string {
	errs := map[string]string{}
	if n := utf8.RuneCountInString(in.Body); n < 1 || n > maxComment {
		errs["body"] = fmt.Sprintf("must be 1 to %d characters", maxComment)
	}
	return errs
}
