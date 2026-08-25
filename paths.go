package volt

import (
	"net/url"
	"strconv"
	"strings"
)

// URLOption augments a generated reverse-URL helper, today with query
// parameters: paths.User(42, volt.Query("tab", "posts")).
type URLOption interface{ apply(*url.Values) }

type queryOption struct{ k, v string }

func (q queryOption) apply(vs *url.Values) { vs.Add(q.k, q.v) }

// Query adds one query parameter to a reverse URL.
func Query(k, v string) URLOption { return queryOption{k, v} }

// URL finishes a generated path: it appends the encoded query built
// from opts. The path itself is assembled by the generated helper with
// the Seg/SegInt/SegWild builders below.
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
// %2E forms: PathEscape leaves dots alone, but as whole segments they
// would change the path shape under cleaning (and ServeMux redirects
// unclean paths), so a built URL must never contain them literally.
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
func SegInt(v int64) string { return strconv.FormatInt(v, 10) }

// SegWild escapes a rest-of-path value, preserving its '/' separators.
// Each element gets Seg's treatment, dot segments included.
func SegWild(s string) string {
	parts := strings.Split(s, "/")
	for i, p := range parts {
		parts[i] = Seg(p)
	}
	return strings.Join(parts, "/")
}
