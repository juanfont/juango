package types

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

// Common errors.
var (
	ErrNotFound       = errors.New("not found")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")
	ErrBadRequest     = errors.New("bad request")
	ErrInternalServer = errors.New("internal server error")
)

// HTTPError represents an error that is surfaced to the user via HTTP.
type HTTPError struct {
	Code int    // HTTP response code to send to client; 0 means 500
	Msg  string // Response body to send to client
	Err  error  // Detailed error to log on the server
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("http error[%d]: %s, %s", e.Code, e.Msg, e.Err)
}

func (e HTTPError) Unwrap() error {
	return e.Err
}

// NewHTTPError creates a new HTTPError.
func NewHTTPError(code int, msg string, err error) HTTPError {
	return HTTPError{Code: code, Msg: msg, Err: err}
}

// WriteHTTPError writes an HTTPError to the response writer.
func WriteHTTPError(w http.ResponseWriter, err error) {
	var herr HTTPError
	if errors.As(err, &herr) {
		http.Error(w, herr.Msg, herr.Code)
		log.Error().Err(herr.Err).Int("code", herr.Code).Msgf("user msg: %s", herr.Msg)
	} else {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Error().Err(err).Int("code", http.StatusInternalServerError).Msg("http internal server error")
	}
}

// HTTPErrorFromStatus creates an HTTPError from an HTTP status code.
func HTTPErrorFromStatus(code int, err error) HTTPError {
	msg := http.StatusText(code)
	if msg == "" {
		msg = "Unknown error"
	}
	return HTTPError{Code: code, Msg: msg, Err: err}
}
