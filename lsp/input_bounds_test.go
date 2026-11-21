package lsp

import (
	"math"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/assert"
	"github.com/Piechutowski/volt/lsp/protocol"
)

func TestDidChangeTakesAReversedRangeAsTheSameSpanAndWarns(t *testing.T) {
	s := NewServer()
	var logged []protocol.LogMessageParams
	ctx := &protocol.Context{Notify: func(method string, params any) {
		if method == protocol.WindowLogMessage {
			logged = append(logged, params.(protocol.LogMessageParams))
		}
	}}
	uri := "file:///reversed.volt"
	opened := protocol.TextDocumentItem{URI: uri, Version: 1, Text: "table users {\n  id int [pk]\n}\n"}
	if err := s.didOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: opened}); err != nil {
		t.Fatal(err)
	}
	if err := s.didChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{ranged(rangeAt(1, 8, 1, 5), "bool")},
	}); err != nil {
		t.Fatal(err)
	}
	if want := "table users {\n  id bool [pk]\n}\n"; s.docGet(uri).Text != want {
		t.Fatalf("text %q, want %q", s.docGet(uri).Text, want)
	}
	if len(logged) != 1 || logged[0].Type != protocol.MessageTypeWarning {
		t.Fatalf("log messages %v, want one warning", logged)
	}
	for _, part := range []string{uri, "version 2", "line 1, character 8", "line 1, character 5"} {
		if !strings.Contains(logged[0].Message, part) {
			t.Errorf("the warning %q does not name %q", logged[0].Message, part)
		}
	}
}

func TestDidChangeWarnsOfNothingForARangeInOrder(t *testing.T) {
	s := NewServer()
	notified := 0
	ctx := &protocol.Context{Notify: func(method string, _ any) {
		if method == protocol.WindowLogMessage {
			notified++
		}
	}}
	uri := "file:///ordered.volt"
	opened := protocol.TextDocumentItem{URI: uri, Version: 1, Text: "table users {\n  id int [pk]\n}\n"}
	if err := s.didOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: opened}); err != nil {
		t.Fatal(err)
	}
	if err := s.didChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			ranged(rangeAt(1, 5, 1, 8), "bool"),
			ranged(rangeAt(1, 5, 1, 5), ""),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if notified != 0 {
		t.Fatalf("%d log messages for ranges in order, want none", notified)
	}
}

func TestPositionsPastTheProtocolsRangeSaturate(t *testing.T) {
	for value, want := range map[int]protocol.UInteger{
		0:                  0,
		7:                  7,
		math.MaxUint32:     math.MaxUint32,
		math.MaxUint32 + 1: math.MaxUint32,
		math.MaxInt:        math.MaxUint32,
	} {
		if got := uintegerSaturated(value, "test"); got != want {
			t.Errorf("uintegerSaturated(%d) = %d, want %d", value, got, want)
		}
	}
	defer func() {
		if !assert.IsFailure(recover()) {
			t.Fatal("a negative value: want an assertion failure")
		}
	}()
	uintegerSaturated(-1, "test")
}

func TestPositionsFarPastTheEndAnswerAtOnce(t *testing.T) {
	doc := NewDocument("file:///far.volt", "table users {\n  id int [pk]\n  \n}\n")
	far := protocol.Position{Line: math.MaxUint32, Character: math.MaxUint32}
	_ = doc.Complete(far)
	_ = doc.docHover(far)
}
