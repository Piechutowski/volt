package volt

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

// Wire formats a query route speaks (spec §V, "Formats"). The format of
// a response is chosen by the request's Accept header, the format of a
// request body by its Content-Type; nothing about the format ever
// appears in the URL, so a path parameter can never be mistaken for
// one.
const (
	MIMEJSON = "application/json"
	MIMEGOB  = "application/x-gob"
)

// Format is one of the two wire formats.
type Format int

const (
	FormatJSON Format = iota // encoding/json: the default, and what curl sees
	FormatGOB                // encoding/gob: the Go-native arm — a client that imports the generated models decodes rows directly
)

// MIME is the media type the format travels under.
func (f Format) MIME() string {
	if f == FormatGOB {
		return MIMEGOB
	}
	return MIMEJSON
}

// Negotiate picks the response format from Accept: the first listed
// media type that is JSON, GOB, `application/*` or `*/*` decides; an
// absent or empty header means JSON; a header that offers nothing the
// server speaks reports false (the caller answers 406).
func Negotiate(r *http.Request) (Format, bool) {
	accept := r.Header.Get("Accept")
	if strings.TrimSpace(accept) == "" {
		return FormatJSON, true
	}
	for _, part := range strings.Split(accept, ",") {
		mt, _, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		switch mt {
		case MIMEGOB:
			return FormatGOB, true
		case MIMEJSON, "application/*", "*/*":
			return FormatJSON, true
		}
	}
	return FormatJSON, false
}

// Render writes v as the response in the negotiated format with status
// 200. A client whose Accept offers neither format gets a 406 through
// the error spine.
func Render(w http.ResponseWriter, r *Request, v any) error {
	return RenderStatus(w, r, http.StatusOK, v)
}

// RenderStatus is Render with an explicit status (201 for a created
// row, for instance). A nil v writes the status and no body.
func RenderStatus(w http.ResponseWriter, r *Request, status int, v any) error {
	f, ok := Negotiate(r.Request)
	if !ok {
		return Error(http.StatusNotAcceptable, "acceptable formats: "+MIMEJSON+", "+MIMEGOB)
	}
	if v == nil {
		w.WriteHeader(status)
		return nil
	}
	w.Header().Set("Content-Type", f.MIME())
	w.WriteHeader(status)
	return Encode(w, f, v)
}

// Encode writes v to w in the given format.
func Encode(w io.Writer, f Format, v any) error {
	if f == FormatGOB {
		return gob.NewEncoder(w).Encode(v)
	}
	return json.NewEncoder(w).Encode(v)
}

// MaxBodyBytes caps a request body Decode reads; a larger body is a
// 413. Four megabytes is more than any params struct needs and less
// than what an attacker would enjoy. Set it before serving.
var MaxBodyBytes int64 = 4 << 20

// Decode reads the request body into v by its Content-Type: JSON when
// the header is absent or names JSON, GOB for application/x-gob. Any
// other type is a 415; an empty or malformed body is a 400; a JSON
// field the struct does not declare is a 400 naming it (a typo must
// not vanish silently); a body over MaxBodyBytes is a 413.
func Decode(r *Request, v any) error {
	f, ok := FormatOf(r.Header.Get("Content-Type"))
	if !ok {
		return Error(http.StatusUnsupportedMediaType, "supported request formats: "+MIMEJSON+", "+MIMEGOB)
	}
	body := http.MaxBytesReader(nil, r.Body, MaxBodyBytes)
	if err := DecodeBody(f, body, v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return Error(http.StatusRequestEntityTooLarge, fmt.Sprintf("request body over %d bytes", MaxBodyBytes))
		}
		return Error(http.StatusBadRequest, "malformed "+f.MIME()+" body: "+err.Error())
	}
	return nil
}

// FormatOf maps a Content-Type header to a format: JSON when the header
// is empty or JSON, GOB for application/x-gob, false for anything else.
func FormatOf(contentType string) (Format, bool) {
	if strings.TrimSpace(contentType) == "" {
		return FormatJSON, true
	}
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return FormatJSON, false
	}
	switch mt {
	case MIMEJSON:
		return FormatJSON, true
	case MIMEGOB:
		return FormatGOB, true
	}
	return FormatJSON, false
}

// DecodeBody reads one value of the format from body. It is the client
// side's decoder too, so both ends of a generated client agree by
// construction. An empty body is an error: a query route with a body
// parameter needs one.
func DecodeBody(f Format, body io.Reader, v any) error {
	if f == FormatGOB {
		return gob.NewDecoder(body).Decode(v)
	}
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty body")
		}
		return err
	}
	return nil
}
