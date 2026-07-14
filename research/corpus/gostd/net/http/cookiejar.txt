package cookiejar // import "net/http/cookiejar"

Package cookiejar implements an in-memory RFC 6265-compliant http.CookieJar.

TYPES

type Jar struct {
	// Has unexported fields.
}
    Jar implements the http.CookieJar interface from the net/http package.

func New(o *Options) (*Jar, error)
    New returns a new cookie jar. A nil *Options is equivalent to a zero
    Options.

func (j *Jar) Cookies(u *url.URL) (cookies []*http.Cookie)
    Cookies implements the Cookies method of the http.CookieJar interface.

    It returns an empty slice if the URL's scheme is not HTTP or HTTPS.

func (j *Jar) SetCookies(u *url.URL, cookies []*http.Cookie)
    SetCookies implements the SetCookies method of the http.CookieJar interface.

    It does nothing if the URL's scheme is not HTTP or HTTPS.

type Options struct {
	// PublicSuffixList is the public suffix list that determines whether
	// an HTTP server can set a cookie for a domain.
	//
	// A nil value is valid and may be useful for testing but it is not
	// secure: it means that the HTTP server for foo.co.uk can set a cookie
	// for bar.co.uk.
	PublicSuffixList PublicSuffixList
}
    Options are the options for creating a new Jar.

type PublicSuffixList interface {
	// PublicSuffix returns the public suffix of domain.
	//
	// TODO: specify which of the caller and callee is responsible for IP
	// addresses, for leading and trailing dots, for case sensitivity, and
	// for IDN/Punycode.
	PublicSuffix(domain string) string

	// String returns a description of the source of this public suffix
	// list. The description will typically contain something like a time
	// stamp or version number.
	String() string
}
    PublicSuffixList provides the public suffix of a domain. For example:
      - the public suffix of "example.com" is "com",
      - the public suffix of "foo1.foo2.foo3.co.uk" is "co.uk", and
      - the public suffix of "bar.pvt.k12.ma.us" is "pvt.k12.ma.us".

    Implementations of PublicSuffixList must be safe for concurrent use by
    multiple goroutines.

    An implementation that always returns "" is valid and may be useful for
    testing but it is not secure: it means that the HTTP server for foo.com can
    set a cookie for bar.com.

    A public suffix list implementation is in the package
    golang.org/x/net/publicsuffix.

