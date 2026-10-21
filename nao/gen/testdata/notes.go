package models

import "fmt"

// PostState is a declared column type stored as string, its values the
// constants below (spec §types).
type PostState string

const (
	// PostStateDraft is not yet visible to readers.
	PostStateDraft PostState = "draft"
	// PostStatePublished is live on the site.
	PostStatePublished PostState = "published"
	PostStateArchived  PostState = "archived"
)

// PostStateKnown refuses a value outside the constants of PostState
// (spec §validators).
func PostStateKnown(v PostState) error {
	switch v {
	case PostStateDraft, PostStatePublished, PostStateArchived:
		return nil
	}
	return fmt.Errorf("%q is not a known post status", string(v))
}
