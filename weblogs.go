// Package weblogs provides access logs for webservers written in go.
package weblogs

import (
  "bytes"
  "fmt"
  "github.com/gorilla/context"
  "io"
  "net/http"
  "net/url"
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
  HasStatus() bool
}

type LogRecord struct {
  T time.Time
  R Snapshot
  W Capture
  Duration time.Duration
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
  // How to write the web logs. nil means the following:
  // time including milliseconds; remote address; method; url; status
  Logger Logger
  // How to get current time.
  Now func() time.Time
}

func (o *Options) writer() io.Writer {
  if o.Writer == nil {
    return os.Stderr
  }
  return o.Writer
}

func (o *Options) logger() Logger {
  if o.Logger == nil {
    return SimpleLogger{}
  }
  return o.Logger
}

func (o *Options) now() func() time.Time {
  if o.Now == nil {
    return time.Now
  }
  return o.Now
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
  return &logHandler{
      handler: handler,
      w: options.writer(),
      logger: options.logger(),
      now: options.now()}
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
  snapshot :=  h.logger.NewSnapshot(r)
  capture := h.logger.NewCapture(w)
  additional := &bytes.Buffer{}
  context.Set(r, kBufferKey, additional)
  startTime := h.now()
  defer func() {
    endTime := h.now()
    err := recover()
    maybeSend500(capture)
    h.logger.Log(
        h.w,
        &LogRecord{
            T: startTime,
            R: snapshot,
            W: capture,
            Duration: endTime.Sub(startTime),
            Extra: additional.String()})
    if err != nil {
      fmt.Fprintf(h.w, "Panic: %v\n", err)
      h.w.Write(debug.Stack())
    }
  }()
  h.handler.ServeHTTP(capture, r)
}

type SimpleSnapshot struct {
  RemoteAddr string
  Method string
  Proto string
  URL *url.URL
}

type SimpleSnapshotFactory struct {
}

func (f SimpleSnapshotFactory) NewSnapshot(r *http.Request) Snapshot {
  urlSnapshot := *r.URL
  return &SimpleSnapshot{
      RemoteAddr: r.RemoteAddr,
      Method: r.Method,
      Proto: r.Proto,
      URL: &urlSnapshot}
}

type SimpleCapture struct {
  http.ResponseWriter
  Status int
  Size int
  statusSet bool
}

func (c *SimpleCapture) Write(b []byte) (int, error) {
  result, err := c.ResponseWriter.Write(b)
  c.Size += result
  c.maybeSetStatus(http.StatusOK)
  return result, err
}

func (c *SimpleCapture) WriteHeader(status int) {
  c.ResponseWriter.WriteHeader(status)
  c.maybeSetStatus(status)
}

func (c *SimpleCapture) HasStatus() bool {
  return c.statusSet
}

func (c *SimpleCapture) maybeSetStatus(status int) {
  if !c.statusSet {
    c.Status = status
    c.statusSet = true
  }
}

type SimpleCaptureFactory struct {
}

func (f SimpleCaptureFactory) NewCapture(w http.ResponseWriter) Capture {
  return &SimpleCapture{ResponseWriter: w}
}

type SimpleLogger struct {
  SimpleSnapshotFactory
  SimpleCaptureFactory
}

func (l SimpleLogger) Log(w io.Writer, log *LogRecord) {
  s := log.R.(*SimpleSnapshot)
  c := log.W.(*SimpleCapture)
  if log.Extra == "" {
    fmt.Fprintf(w, "%s %s %s %s %d %d\n",
        log.T.Format("01/02/2006 15:04:05.999999"),
        s.RemoteAddr,
        s.Method,
        s.URL,
        c.Status,
        log.Duration / time.Millisecond)
  } else {
    fmt.Fprintf(w, "%s %s %s %s %d %d%s\n",
        log.T.Format("01/02/2006 15:04:05.999999"),
        s.RemoteAddr,
        s.Method,
        s.URL,
        c.Status,
        log.Duration / time.Millisecond,
        log.Extra)
  }
}

type ApacheCommonLogger struct {
  SimpleLogger
}

func maybeSend500(c Capture) {
  if !c.HasStatus() {
    sendError(c, http.StatusInternalServerError)
  }
}

func sendError(w http.ResponseWriter, status int) {
  http.Error(w, fmt.Sprintf("%d %s", status, http.StatusText(status)), status)
}

type nilWriter struct {
}

func (w nilWriter) Write(p []byte) (n int, err error) {
  return len(p), nil
}
