package main

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectRefusesBadFlags(t *testing.T) {
	for args, want := range map[string]string{
		"-sizes 2,x":          `"x" is not a table count`,
		"-sizes 0":            `"0" is not a table count`,
		"-profile 1s":         "-profile needs -out",
		"-columns notanumber": "invalid value",
	} {
		err := commandCollect(strings.Fields(args))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("collect %s = %v, want %q", args, err, want)
		}
	}
}

// TestServeStopsWithAStreamOpen proves a shutdown ends the open event
// streams instead of waiting out the five-second grace period, while a
// request in flight still finishes on its own context.
func TestServeStopsWithAStreamOpen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan error, 1)
	go func() { done <- serveUntil(ctx, ln, filepath.Join(t.TempDir(), "v.db")) }()
	base := "http://" + ln.Addr().String()
	var resp *http.Response
	for range 100 {
		if resp, err = http.Get(base + "/events"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET /events = %d", resp.StatusCode)
	}
	start := time.Now()
	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serve after the stop = %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("serve still waits on the open stream four seconds after the stop")
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("shutdown took %v with a stream open, want well under a second", took)
	}
}
