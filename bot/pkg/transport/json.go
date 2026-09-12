package transport

import (
	"encoding/json"
	"io"
	"net/http"
)

// maxRequestBytes caps a pairing request. It only carries a short code, so
// anything larger is a mistake or an attempt to make the bot allocate.
const maxRequestBytes = 4 * 1024

// decodeJSON reads one JSON object from a request body, and refuses anything
// that is not exactly that.
//
// DisallowUnknownFields is on so a capture build that sends a field this
// version does not know is told, rather than having it silently dropped and
// then wondering why pairing behaved differently than expected.
func decodeJSON(r *http.Request, into any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(into); err != nil {
		return err
	}
	// A body with a second document in it is ambiguous about which one was
	// meant, so it is refused rather than half-read.
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return errTrailingContent
	}
	return nil
}

type transportError string

func (e transportError) Error() string { return string(e) }

const errTrailingContent = transportError("the request body carries more than one document")

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	// Nothing here is cacheable, and a pairing response least of all.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
