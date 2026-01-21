// Package middleware provides common HTTP middleware for Go web applications.
package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logging returns a middleware that logs HTTP requests using zerolog.
func Logging(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := newResponseWriter(w)
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			var event *zerolog.Event
			switch {
			case wrapped.statusCode >= 500:
				event = logger.Error()
			case wrapped.statusCode >= 400:
				event = logger.Warn()
			default:
				event = logger.Debug()
			}

			event.
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapped.statusCode).
				Dur("duration", duration).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Msg("HTTP request")
		})
	}
}

// LoggingFunc returns a middleware function for use with gorilla/mux.
func LoggingFunc(logger zerolog.Logger) func(http.Handler) http.Handler {
	return Logging(logger)
}
