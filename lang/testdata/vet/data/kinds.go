package data

import "fmt"

// PostStatus is a declared column type stored as string, its values the
// constants below (spec §types).
type PostStatus string

const (
	PostStatusDraft     PostStatus = "draft"
	PostStatusPublished PostStatus = "published"
)

// PostStatusKnown refuses a value outside the constants of PostStatus
// (spec §validators).
func PostStatusKnown(v PostStatus) error {
	switch v {
	case PostStatusDraft, PostStatusPublished:
		return nil
	}
	return fmt.Errorf("%q is not a known post status", string(v))
}

// CoreKind is a declared column type stored as string, its values the
// constants below (spec §types).
type CoreKind string

const (
	CoreKindWidget CoreKind = "widget"
)

// CoreKindKnown refuses a value outside the constants of CoreKind
// (spec §validators).
func CoreKindKnown(v CoreKind) error {
	switch v {
	case CoreKindWidget:
		return nil
	}
	return fmt.Errorf("%q is not a known core kind", string(v))
}
