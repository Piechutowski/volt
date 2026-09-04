package volt

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is the wire side of a generated client package (spec §V4.10):
// the generated methods build typed URLs and bodies and hand them here.
// It speaks the Formats rules from the other end — Accept and
// Content-Type from Format, the reply decoded by its Content-Type — so
// both ends of a route agree by construction.
type Client struct {
	Base   string       // server origin, e.g. "http://localhost:8888"; no trailing slash
	HTTP   *http.Client // nil means http.DefaultClient
	Format Format       // wire format for request bodies and Accept; FormatJSON by default
}

// Do issues one request. A non-nil in is encoded as the body; a non-nil
// out receives the decoded reply. A reply outside 2xx is returned as an
// HTTPError carrying its status and body text, so errors.Is(err,
// volt.ErrNotFound) holds for a 404 and errors.As recovers any status.
func (c *Client) Do(ctx context.Context, method, url string, in, out any) error {
	resp, err := c.send(ctx, method, url, in)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Error(resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	f, ok := FormatOf(resp.Header.Get("Content-Type"))
	if !ok {
		return fmt.Errorf("volt client: %s %s answered %q, which is neither JSON nor GOB", method, url, resp.Header.Get("Content-Type"))
	}
	return DecodeBody(f, resp.Body, out)
}

// Raw issues a request to a controller route and returns the response
// for the caller to read and close: Volt does not know what a
// hand-written action writes.
func (c *Client) Raw(ctx context.Context, method, url string) (*http.Response, error) {
	return c.send(ctx, method, url, nil)
}

func (c *Client) send(ctx context.Context, method, url string, in any) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		var buf bytes.Buffer
		if err := Encode(&buf, c.Format, in); err != nil {
			return nil, err
		}
		body = &buf
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+url, body)
	if err != nil {
		return nil, err
	}
	if in != nil {
		req.Header.Set("Content-Type", c.Format.MIME())
	}
	req.Header.Set("Accept", c.Format.MIME())
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	return hc.Do(req)
}
