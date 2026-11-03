package main

import (
	"context"
	"io"
	"net/http"
	"slices"
	"testing"

	"github.com/Piechutowski/volt/examples/pipelines/app/client"
)

// TestPipelinesGuard calls both routes with and without the key: the
// api pipeline runs on both, the vault pipeline only under /vault, and
// RequireKey refuses the request without the key before the handler.
func TestPipelinesGuard(t *testing.T) {
	base, err := serve()
	if err != nil {
		t.Fatal(err)
	}
	root := &Root{client: client.New(base)}
	root.client.HTTP = &http.Client{Transport: keyed{root: root}}
	ctx := context.Background()
	for _, c := range []struct {
		key    bool
		call   func(context.Context) (*http.Response, error)
		status int
		plugs  []string
	}{
		{false, root.client.Open, 200, []string{"Stamp"}},
		{false, root.client.VaultSecret, 401, []string{"Stamp", "RequireKey"}},
		{true, root.client.VaultSecret, 200, []string{"Stamp", "RequireKey"}},
	} {
		root.sendKey = c.key
		resp, err := c.call(ctx)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != c.status || !slices.Equal(resp.Header.Values("X-Plugs"), c.plugs) || resp.Header.Get("X-Request-Id") == "" {
			t.Errorf("%s with key %v = %d, plugs %v, request id %q", resp.Request.URL.Path, c.key, resp.StatusCode, resp.Header.Values("X-Plugs"), resp.Header.Get("X-Request-Id"))
		}
	}
}
