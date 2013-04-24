// Package weblogs provides access logs for webservers written in go.
package weblogs

import (
  "bytes"
  "fmt"
  "github.com/gorilla/context"
  "io"
  "log"
  "net/http"
  "os"
  "runtime/debug"
)

type contextKeyType int

const (
  kBufferKey contextKeyType = iota
)

var (
  kLog *log.Logger
)

// Handler wraps handler creating access logs. Returned handler must be
// wrapped by context.ClearHandler.
func Handler(handler http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    remoteAddr, method, url := r.RemoteAddr, r.Method, r.URL
    additional := &bytes.Buffer{}
    context.Set(r, kBufferKey, additional)
    writer := &statusWriter{ResponseWriter: w}
    defer func() {
      err := recover()
      writer.MaybeSend500()
      var buf bytes.Buffer
      if additional.Len() == 0 {
        fmt.Fprintf(
            &buf, "%s %s %s %d\n", remoteAddr, method, url, writer.Status())
      } else {
        fmt.Fprintf(
            &buf, "%s %s %s %d%s\n", remoteAddr, method, url, writer.Status(), additional.String())
      }
      if err != nil {
        fmt.Fprintf(&buf, "Panic: %v\n", err)
        buf.Write(debug.Stack())
      }
      kLog.Print(buf.String())
    }()
    handler.ServeHTTP(writer, r)
  })
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

func init() {
  kLog = log.New(os.Stderr, "", log.LstdFlags | log.Lmicroseconds)
}
