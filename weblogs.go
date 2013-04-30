// Package weblogs provides access logs for webservers written in go.
package weblogs

import (
  "bytes"
  "fmt"
  "github.com/gorilla/context"
  "io"
  "net/http"
  "os"
  "runtime/debug"
  "time"
)

type contextKeyType int

const (
  kBufferKey contextKeyType = iota
)

var (
  kNilWriter nilWriter
)

type Snapshot interface{}

type Capture interface {
  http.ResponseWriter
  Status() int
  IsStatusSet() bool
}

type LogRecord struct {
  T time.Time
  R Snapshot
  W Capture
  Extra string
}

type Logger interface {
  NewSnapshot(r *http.Request) Snapshot
  NewCapture(w http.ResponseWriter) Capture
  Log(w io.Writer, record *LogRecord)
}
  
// Options specifies options for writing to access logs.
type Options struct {
  // Where to write the web logs. nil means write to stderr,
  Writer io.Writer
}

func (o *Options) writer() io.Writer {
  if o.Writer == nil {
    return os.Stderr
  }
  return o.Writer
}

// Handler wraps a handler creating access logs. Returned handler must be
// wrapped by context.ClearHandler.
func Handler(handler http.Handler) http.Handler {
  return HandlerWithOptions(handler, nil)
}

// HandlerWithOptions wraps a handler creating access logs and allows caller to
// configure how access logs are written. Returned handler must be
// wrapped by context.ClearHandler.
func HandlerWithOptions(
    handler http.Handler, options *Options) http.Handler {
  if options == nil {
    options = &Options{}
  }
  // TODO: fix.
  return &logHandler{handler: handler}
}

// Writer returns a writer whereby the caller can add additional information
// to the current log entry. If the handler calling this is not wrapped by
// the Handler() method, then writing to the returned io.Writer does
// nothing.
func Writer(r *http.Request) io.Writer {
  value := context.Get(r, kBufferKey)
  if value == nil {
    return kNilWriter
  }
  return value.(*bytes.Buffer)
}

type logHandler struct {
  handler http.Handler
  w io.Writer
  logger Logger
  now func() time.Time
}

func (h *logHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  now := h.now()
  snapshot :=  h.logger.NewSnapshot(r)
  capture := h.logger.NewCapture(w)
  additional := &bytes.Buffer{}
  context.Set(r, kBufferKey, additional)
  defer func() {
    err := recover()
    maybeSend500(capture)
    h.logger.Log(h.w, &LogRecord{T: now, R: snapshot, W: capture, Extra: additional.String()})
    if err != nil {
      fmt.Fprintf(h.w, "Panic: %v\n", err)
      h.w.Write(debug.Stack())
    }
  }()
  h.handler.ServeHTTP(capture, r)
}

func maybeSend500(w Capture) {
  if !w.IsStatusSet() {
    sendError(w, http.StatusInternalServerError)
  }
}

func sendError(w http.ResponseWriter, status int) {
  http.Error(w, fmt.Sprintf("%d %s", status, http.StatusText(status)), status)
}

type statusWriter struct {
  http.ResponseWriter
  status int
  statusSet bool
}

func (w *statusWriter) MaybeSend500() {
  if !w.statusSet {
    sendError(w, http.StatusInternalServerError)
  }
}

func (w *statusWriter) Write(b []byte) (int, error) {
  result, err := w.ResponseWriter.Write(b)
  w.maybeSetStatus(http.StatusOK)
  return result, err
}

func (w *statusWriter) WriteHeader(status int) {
  w.ResponseWriter.WriteHeader(status)
  w.maybeSetStatus(status)
}

func (w *statusWriter) maybeSetStatus(status int) {
  if !w.statusSet {
    w.status = status
    w.statusSet = true
  }
}

func (w *statusWriter) Status() int {
  return w.status
}

type nilWriter struct {
}

func (w nilWriter) Write(p []byte) (n int, err error) {
  return len(p), nil
}
