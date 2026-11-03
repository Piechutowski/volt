package main

import (
	"strings"
	"testing"
)

func TestCollectRefusesBadFlags(t *testing.T) {
	for args, want := range map[string]string{
		"-sizes 2,x":          `"x" is not a table count`,
		"-sizes 0":            `"0" is not a table count`,
		"-profile 1s":         "-profile needs -out",
		"-columns notanumber": "invalid value",
	} {
		err := collect(strings.Fields(args))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("collect %s = %v, want %q", args, err, want)
		}
	}
}
