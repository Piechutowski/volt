package token

import (
	"math"
	"testing"
)

func TestPositionsPastThirtyTwoBitsStayExact(t *testing.T) {
	past := math.MaxInt32 + 10
	position := NewFile("wide.volt", "").At(past, past+1, past+2)
	if position.Offset() != past || position.Line() != past+1 || position.Column() != past+2 {
		t.Fatalf("offset %d, line %d, column %d", position.Offset(), position.Line(), position.Column())
	}
	advanced := position.Advance(past)
	if advanced.Offset() != 2*past || advanced.Column() != 2*past+2 {
		t.Fatalf("advanced: offset %d, column %d", advanced.Offset(), advanced.Column())
	}
	chunked := NewChunk("whole.volt", "", past, past+1).At(past, 1, 1)
	if chunked.Offset() != 2*past || chunked.Line() != past+1 {
		t.Fatalf("in a chunk: offset %d, line %d", chunked.Offset(), chunked.Line())
	}
}

func TestTokenEndCountsLinesAndColumns(t *testing.T) {
	file := NewFile("text.volt", "ab\ncd")
	end := Token{Kind: STRING, Len: 5, Pos: file.At(0, 1, 1)}.End()
	if end.Offset() != 5 || end.Line() != 2 || end.Column() != 3 {
		t.Fatalf("offset %d, line %d, column %d", end.Offset(), end.Line(), end.Column())
	}
}
