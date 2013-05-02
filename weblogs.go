// Copyright 2013 Travis Keep. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or
// at http://opensource.org/licenses/BSD-3-Clause.

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
  "strings"
  "time"
)

type contextKeyType int

const (
  kBufferKey contextKeyType = iota
)

// Snapshot represents a snapshot of an HTTP request.
type Snapshot interface{}

// Capture captures a server response. Implementations delegate to an
// underlying ResponseWriter.
type Capture interface {
  http.ResponseWriter
  // HasStatus returns true if server has sent a status. False means that
  // server failed to send a response.
  HasStatus() bool
}

// LogRecord represents a single entry in the access logs.
type LogRecord struct {
  // The time request was received.
  T time.Time
  // The request snapshot
  R Snapshot
  // The capture of the response
  W Capture
  // Time spent processing the request
  Duration time.Duration
  // Additional information added with the Writer method.
  Extra string
}

// Logger represents an access log format. Clients are free to provide their
// own implementations.
type Logger interface {
  // NewSnapshot creates a new snapshot of a request.
  NewSnapshot(r *http.Request) Snapshot
  // NewCapture creates a new capture for capturing a response. w is the
  // original ResponseWriter.
  NewCapture(w http.ResponseWriter) Capture
  // Log writes the log record.
  Log(w io.Writer, record *LogRecord)
}
  
// Options specifies options for writing to access logs.
type Options struct {
  // Where to write the web logs. nil means write to stderr,
  Writer io.Writer
  // How to write the web logs. nil means use SimpleLogger.
  Logger Logger
  // How to get current time. nil means use time.Now(). This field is used
  // for testing purposes.
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

// Handler wraps a handler creating access logs. Access logs are written to
// stderr using SimpleLogger. Returned handler must be wrapped by
// context.ClearHandler.
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
    return nilWriter{}
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

// ApacheCommonSnapshot provides a request snapshot for apache common
// access logs.
type ApacheCommonSnapshot struct {
  // Copied from Request.RemoteAddr
  RemoteAddr string
  // Copied from Request.Method
  Method string
  // Copied from Request.Proto
  Proto string
  // Copied from Request.URL
  URL *url.URL
}

func NewApacheCommonSnapshot(r *http.Request) ApacheCommonSnapshot {
  urlSnapshot := *r.URL
  return ApacheCommonSnapshot{
      RemoteAddr: r.RemoteAddr,
      Method: r.Method,
      Proto: r.Proto,
      URL: &urlSnapshot}
}

// ApacheCombinedSnapshot provides a request snapshot for apache combined
// access logs.
type ApacheCombinedSnapshot struct {
  ApacheCommonSnapshot
  Referer string
  UserAgent string
}

func NewApacheCombinedSnapshot(r *http.Request) ApacheCombinedSnapshot {
  return ApacheCombinedSnapshot{
      ApacheCommonSnapshot: NewApacheCommonSnapshot(r),
      Referer: r.Referer(),
      UserAgent: r.UserAgent()}
}

// SimpleCapture provides a capture of a response that includes the http
// status code and the size of the response.
type SimpleCapture struct {
  // The underlying ResponseWriter
  http.ResponseWriter
  // The HTTP status code shows up here.
  Status int
  // The size of the response in bytes shows up here.
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

// SimpleLogger provides access logs with the following columns:
// date, remote address, method, URI, status, time elapsed milliseconds,
// followed by any additional information provided via the Writer method.
type SimpleLogger struct {
}

func (l SimpleLogger) NewSnapshot(r *http.Request) Snapshot {
  snapshot := NewApacheCommonSnapshot(r)
  return &snapshot
}

func (l SimpleLogger) NewCapture(w http.ResponseWriter) Capture {
  return &SimpleCapture{ResponseWriter: w}
}

func (l SimpleLogger) Log(w io.Writer, log *LogRecord) {
  s := log.R.(*ApacheCommonSnapshot)
  c := log.W.(*SimpleCapture)
  fmt.Fprintf(w, "%s %s %s %s %d %d%s\n",
      log.T.Format("01/02/2006 15:04:05"),
      strings.Split(s.RemoteAddr, ":")[0],
      s.Method,
      s.URL,
      c.Status,
      log.Duration / time.Millisecond,
      log.Extra)
}

// ApacheCommonLogger provides access logs in apache common log format.
type ApacheCommonLogger struct {
}

func (l ApacheCommonLogger) NewSnapshot(r *http.Request) Snapshot {
  snapshot := NewApacheCommonSnapshot(r)
  return &snapshot
}

func (l ApacheCommonLogger) NewCapture(w http.ResponseWriter) Capture {
  return &SimpleCapture{ResponseWriter: w}
}

func (l ApacheCommonLogger) Log(w io.Writer, log *LogRecord) {
  s := log.R.(*ApacheCommonSnapshot)
  c := log.W.(*SimpleCapture)
  fmt.Fprintf(w, "%s - %s [%s] \"%s %s %s\" %d %d\n",
        strings.Split(s.RemoteAddr, ":")[0],
        ApacheUser(s.URL.User),
        log.T.Format("02/Jan/2006:15:04:05 -0700"),
        s.Method,
        s.URL.RequestURI(),
        s.Proto,
        c.Status,
        c.Size)
}

// ApacheCombinedLogger provides access logs in apache combined log format.
type ApacheCombinedLogger struct {
}

func (l ApacheCombinedLogger) NewSnapshot(r *http.Request) Snapshot {
  snapshot := NewApacheCombinedSnapshot(r)
  return &snapshot
}

func (l ApacheCombinedLogger) NewCapture(w http.ResponseWriter) Capture {
  return &SimpleCapture{ResponseWriter: w}
}

func (l ApacheCombinedLogger) Log(w io.Writer, log *LogRecord) {
  s := log.R.(*ApacheCombinedSnapshot)
  c := log.W.(*SimpleCapture)
  fmt.Fprintf(w, "%s - %s [%s] \"%s %s %s\" %d %d \"%s\" \"%s\"\n",
        strings.Split(s.RemoteAddr, ":")[0],
        ApacheUser(s.URL.User),
        log.T.Format("02/Jan/2006:15:04:05 -0700"),
        s.Method,
        s.URL.RequestURI(),
        s.Proto,
        c.Status,
        c.Size,
        s.Referer,
        s.UserAgent)
}


// ApacheUser is a utility method for Logger implementations that formats
// user info in a request for apache logging.
func ApacheUser(user *url.Userinfo) string {
  result := "-"
  if user != nil {
    if name := user.Username(); name != "" {
      result = name
    }
  }
  return result
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
