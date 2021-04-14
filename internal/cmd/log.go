package cmd

import (
	"net/http"

	"github.com/lthibault/log"
	"github.com/sirupsen/logrus"
)

// Logger is a wrapper around github.com/lthibault/log with
// convenience methods.
type Logger struct{ log.Logger }

func (l Logger) With(v log.Loggable) *Logger               { return &Logger{l.Logger.With(v)} }
func (l Logger) WithError(err error) *Logger               { return &Logger{l.Logger.WithError(err)} }
func (l Logger) WithField(s string, v interface{}) *Logger { return &Logger{l.Logger.WithField(s, v)} }
func (l Logger) WithFields(log logrus.Fields) *Logger      { return &Logger{l.Logger.WithFields(log)} }

func (l *Logger) WithRequest(r *http.Request) *Logger {
	return l.With(log.F{
		"method": r.Method,
		"path":   r.URL.Path,
	})
}

// // Log retrieves a logger from a context.  It panics if the logger was not set.
// func Log(ctx context.Context) Logger {
// 	return ctx.Value(keyLogger{}).(Logger)
// }

// type keyLogger struct{}

// type logWriter struct {
// 	i int
// 	Logger
// 	http.ResponseWriter
// }

// func (w *logWriter) Write(b []byte) (int, error) {
// 	n, err := w.ResponseWriter.Write(b)
// 	w.i += n
// 	return n, err
// }

// func (w *logWriter) WriteHeader(statusCode int) {
// 	w.Logger = w.Logger.WithField("status", statusCode)
// 	w.ResponseWriter.WriteHeader(statusCode)
// }

// func (w *logWriter) Bind(r *http.Request) *http.Request {
// 	return r.WithContext(context.WithValue(r.Context(),
// 		keyLogger{},
// 		w.Logger.WithRequest(r)))
// }
