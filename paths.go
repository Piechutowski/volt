package volt

import (
	"net/url"
	"strconv"
	"strings"
)

// URLOption augments a generated reverse-URL helper with query
// parameters.
type URLOption interface{ apply(*url.Values) }

type queryOption struct{ k, v string }

func (q queryOption) apply(vs *url.Values) { vs.Add(q.k, q.v) }

// Query adds one query parameter to a reverse URL.
func Query(k, v string) URLOption { return queryOption{k, v} }

// URL finishes a generated path: it appends the encoded query built
// from opts.
func URL(path string, opts ...URLOption) string {
	if len(opts) == 0 {
		return path
	}
	vs := url.Values{}
	for _, o := range opts {
		o.apply(&vs)
	}
	return path + "?" + vs.Encode()
}

// Seg escapes one string path parameter. "." and ".." escape to their
// %2E forms, so a built URL never contains them literally.
func Seg(s string) string {
	switch s {
	case ".":
		return "%2E"
	case "..":
		return "%2E%2E"
	}
	return url.PathEscape(s)
}

// SegInt renders an integer path parameter.
func SegInt(v int) string { return strconv.Itoa(v) }

// SegWild escapes a rest-of-path value, preserving its '/' separators
// but dropping its empty elements, a trailing one aside: the clean form
// ServeMux would redirect to (spec §route-names-and-reverse-urls). Each
// element gets Seg's treatment, dot segments included.
func SegWild(s string) string {
	parts := strings.Split(s, "/")
	kept := make([]string, 0, len(parts))
	for i, p := range parts {
		if p == "" && i < len(parts)-1 {
			continue
		}
		kept = append(kept, Seg(p))
	}
	return strings.Join(kept, "/")
}
