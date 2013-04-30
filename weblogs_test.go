package weblogs_test

import (
  "bytes"
  "fmt"
  "github.com/keep94/weblogs"
  "net/http"
  "net/url"
  "testing"
  "time"
)

var (
  kNilResponseWriter nilResponseWriter
  kTime = time.Date(2013, time.March, 23, 13, 14, 15, 123456789, time.UTC)
)

func now() time.Time {
  return kTime
}

func TestNormalLogs(t *testing.T) {
  buf := &bytes.Buffer{}
  handler := weblogs.HandlerWithOptions(
      &handler{Status: 321}, &weblogs.Options{Writer: buf, Now: now})
  handler.ServeHTTP(
      kNilResponseWriter,
      newRequest("192.168.5.1", "GET", "/foo/bar?query=tall"))
  actual := buf.String()
  expected := "03/23/2013 13:14:15.123456 192.168.5.1 GET /foo/bar?query=tall 321\n"
  verifyLogs(t, expected, actual)
}

func TestAppendedLogs(t *testing.T) {
  buf := &bytes.Buffer{}
  handler := weblogs.HandlerWithOptions(
      &handler{Status: 321, LogExtra: "behere"}, &weblogs.Options{Writer: buf, Now: now})
  handler.ServeHTTP(
      kNilResponseWriter,
      newRequest("192.168.5.1", "GET", "/foo/bar?query=tall"))
  actual := buf.String()
  expected := "03/23/2013 13:14:15.123456 192.168.5.1 GET /foo/bar?query=tall 321 behere\n"
  verifyLogs(t, expected, actual)
}

func TestSend500OnNoOutput(t *testing.T) {
  buf := &bytes.Buffer{}
  handler := weblogs.HandlerWithOptions(
      &handler{LogExtra: "behere"}, &weblogs.Options{Writer: buf, Now: now})
  w := &spyResponseWriter{}
  handler.ServeHTTP(
      w,
      newRequest("192.168.5.1", "GET", "/foo/bar?query=tall"))
  actual := buf.String()
  expected := "03/23/2013 13:14:15.123456 192.168.5.1 GET /foo/bar?query=tall 500 behere\n"
  verifyLogs(t, expected, actual)
  if w.Status != 500 {
    t.Errorf("Expected 500 error to be sent, but %d was sent.", w.Status)
  }
}

func TestUnwrappedCallToWriter(t *testing.T) {
  // logging extra should should be silently ignored.
  handler := &handler{LogExtra: "behere"}
  handler.ServeHTTP(
      kNilResponseWriter,
      newRequest("192.168.5.1", "GET", "/foo/bar?query=tall"))
}

func verifyLogs(t *testing.T, expected, actual string) {
  if expected != actual {
    t.Errorf("Expected %s, got %s", expected, actual)
  }
}

func newRequest(remoteAddr, method, urlStr string) *http.Request {
  u, _ := url.Parse(urlStr)
  return &http.Request{
    RemoteAddr: remoteAddr,
    Method: method,
    URL: u}
}

type handler struct {
  Status int
  LogExtra string
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  if h.Status != 0 {
    http.Error(w, "Error", h.Status)
  }
  if h.LogExtra != "" {
    fmt.Fprintf(weblogs.Writer(r), " %s", h.LogExtra)
  }
}

type nilResponseWriter struct {
}

func (w nilResponseWriter) Write(b []byte) (n int, err error) {
  return len(b), nil
}

func (w nilResponseWriter) WriteHeader(status int) {
}

func (w nilResponseWriter) Header() http.Header {
  return http.Header{}
}

type spyResponseWriter struct {
  nilResponseWriter
  Status int
}

func (w *spyResponseWriter) WriteHeader(status int) {
  w.Status = status
}
  
