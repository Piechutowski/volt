package models

import "fmt"

// CommentableType is a declared column type stored as string, its values the
// constants below (spec §types).
type CommentableType string

const (
	CommentableTypePost  CommentableType = "post"
	CommentableTypePhoto CommentableType = "photo"
	CommentableTypeVideo CommentableType = "video"
)

// CommentableTypeKnown refuses a value outside the constants of CommentableType
// (spec §validators).
func CommentableTypeKnown(v CommentableType) error {
	switch v {
	case CommentableTypePost, CommentableTypePhoto, CommentableTypeVideo:
		return nil
	}
	return fmt.Errorf("%q is not a known commentable type", string(v))
}
