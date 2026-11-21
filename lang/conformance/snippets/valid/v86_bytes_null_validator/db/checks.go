package db

import (
	"errors"

	"github.com/Piechutowski/volt/nao/rt"
)

// BodySmall refuses a body over a kilobyte (spec/validators).
func BodySmall(b []byte) error {
	if len(b) > 1024 {
		return errors.New("body over a kilobyte")
	}
	return nil
}

// ThumbSmall refuses a thumbnail over a kilobyte; NULL passes (spec/validators).
func ThumbSmall(t rt.Null[[]byte]) error {
	if t.Valid && len(t.V) > 1024 {
		return errors.New("thumbnail over a kilobyte")
	}
	return nil
}
