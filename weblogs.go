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
  "github.com/keep94/weblogs/loggers"
  "io"
  "net/http"
  "os"
  "runtime/debug"
  "sync"
  "time"
)

type contextKeyType int

const (
  kBufferKey contextKeyType = iota
  kValuesKey
)

var (
  kNoOptions = &Options{}
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
  // Key-value pairs to be logged.
  Values map[interface{}]interface{}
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
  // How to write the web logs. nil means SimpleLogger().
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
    return simpleLogger{}
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
// stderr using SimpleLogger(). Returned handler must be wrapped by
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
    options = kNoOptions
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

// Values returns the current key-value pairs to be logged.
// If the handler calling this is not wrapped by the Handler() method,
// then this method returns nil.
func Values(r *http.Request) map[interface{}]interface{} {
  instance := context.Get(r, kValuesKey)
  if instance == nil {
    return nil
  }
  return instance.(map[interface{}]interface{})
}

type logHandler struct {
  // mutex protects the w field.
  mutex sync.Mutex
  handler http.Handler
  w io.Writer
  logger Logger
  now func() time.Time
}

func (h *logHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  snapshot :=  h.logger.NewSnapshot(r)
  capture := h.logger.NewCapture(w)
  additional := &bytes.Buffer{}
  values := make(map[interface{}]interface{})
  context.Set(r, kBufferKey, additional)
  context.Set(r, kValuesKey, values)
  startTime := h.now()
  defer func() {
    endTime := h.now()
    err := recover()
    maybeSend500(capture)
    h.writeLogRecord(
        &LogRecord{
            T: startTime,
            R: snapshot,
            W: capture,
            Duration: endTime.Sub(startTime),
            Extra: additional.String(),
            Values: values})
    if err != nil {
      h.writePanic(err, debug.Stack())
    }
  }()
  h.handler.ServeHTTP(capture, r)
}

func (h *logHandler) writeLogRecord(logRecord *LogRecord) {
  h.mutex.Lock()
  defer h.mutex.Unlock()
  h.logger.Log(h.w, logRecord)
}

func (h *logHandler) writePanic(
    panicError interface{}, debugStack []byte) {
  h.mutex.Lock()
  defer h.mutex.Unlock()
  fmt.Fprintf(h.w, "Panic: %v\n", panicError)
  h.w.Write(debugStack)
}
    
// SimpleLogger provides access logs with the following columns:
// date, remote address, method, URI, status, time elapsed milliseconds,
// followed by any additional information provided via the Writer method.
func SimpleLogger() Logger {
  return simpleLogger{}
}

// ApacheCommonLogger provides access logs in apache common log format.
func ApacheCommonLogger() Logger {
  return apacheCommonLogger{}
}

// ApacheCombinedLogger provides access logs in apache combined log format.
func ApacheCombinedLogger() Logger {
  return apacheCombinedLogger{}
}

type loggerBase struct {
}

func (l loggerBase) NewSnapshot(r *http.Request) Snapshot {
  return loggers.NewSnapshot(r)
}

func (l loggerBase) NewCapture(w http.ResponseWriter) Capture {
  return &loggers.Capture{ResponseWriter: w}
}

type simpleLogger struct {
  loggerBase
}

func (l simpleLogger) Log(w io.Writer, log *LogRecord) {
  s := log.R.(*loggers.Snapshot)
  c := log.W.(*loggers.Capture)
  fmt.Fprintf(w, "%s %s %s %s %d %d%s\n",
      log.T.Format("01/02/2006 15:04:05"),
      loggers.StripPort(s.RemoteAddr),
      s.Method,
      s.URL,
      c.Status(),
      log.Duration / time.Millisecond,
      log.Extra)
}

type apacheCommonLogger struct {
  loggerBase
}

func (l apacheCommonLogger) Log(w io.Writer, log *LogRecord) {
  s := log.R.(*loggers.Snapshot)
  c := log.W.(*loggers.Capture)
  fmt.Fprintf(w, "%s - %s [%s] \"%s %s %s\" %d %d\n",
        loggers.StripPort(s.RemoteAddr),
        loggers.ApacheUser(s.URL.User),
        log.T.Format("02/Jan/2006:15:04:05 -0700"),
        s.Method,
        s.URL.RequestURI(),
        s.Proto,
        c.Status(),
        c.Size())
}

type apacheCombinedLogger struct {
  loggerBase
}

func (l apacheCombinedLogger) Log(w io.Writer, log *LogRecord) {
  s := log.R.(*loggers.Snapshot)
  c := log.W.(*loggers.Capture)
  fmt.Fprintf(w, "%s - %s [%s] \"%s %s %s\" %d %d \"%s\" \"%s\"\n",
        loggers.StripPort(s.RemoteAddr),
        loggers.ApacheUser(s.URL.User),
        log.T.Format("02/Jan/2006:15:04:05 -0700"),
        s.Method,
        s.URL.RequestURI(),
        s.Proto,
        c.Status(),
        c.Size(),
        s.Referer,
        s.UserAgent)
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
