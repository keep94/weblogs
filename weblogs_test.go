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

func TestNormalLogs(t *testing.T) {
  buf := &bytes.Buffer{}
  clock := &clock{Time: kTime}
  handler := weblogs.HandlerWithOptions(
      &handler{Status: 321, Clock: clock, ElapsedMillis: 387},
      &weblogs.Options{Writer: buf, Now: clock.Now()})
  handler.ServeHTTP(
      kNilResponseWriter,
      newRequest("192.168.5.1", "GET", "/foo/bar?query=tall"))
  expected := "03/23/2013 13:14:15.123456 192.168.5.1 GET /foo/bar?query=tall 321 387\n"
  verifyLogs(t, expected, buf.String())
}

func TestCommonLogs(t *testing.T) {
  buf := &bytes.Buffer{}
  clock := &clock{Time: kTime}
  handler := weblogs.HandlerWithOptions(
      &handler{Status: 321, Message: "1234567"},
      &weblogs.Options{
          Writer: buf,
          Logger: weblogs.ApacheCommonLogger{},
          Now: clock.Now()})
  request := newRequest("192.168.5.1", "GET", "/foo/bar?query=tall")
  request.URL.User = url.User("fred")
  handler.ServeHTTP(
      kNilResponseWriter,
      request)
  expected := "192.168.5.1 - fred [23/Mar/2013:13:14:15 +0000] \"GET /foo/bar?query=tall HTTP/1.0\" 321 7\n"
  verifyLogs(t, expected, buf.String())
}

func TestCombinedLogs(t *testing.T) {
  buf := &bytes.Buffer{}
  clock := &clock{Time: kTime}
  handler := weblogs.HandlerWithOptions(
      &handler{Status: 321, Message: "1234567"},
      &weblogs.Options{
          Writer: buf,
          Logger: weblogs.ApacheCombinedLogger{},
          Now: clock.Now()})
  request := newRequest("192.168.5.1", "GET", "/foo/bar?query=tall")
  request.URL.User = url.User("fred")
  request.Header = make(http.Header)
  request.Header.Set("Referer", "referer")
  request.Header.Set("User-Agent", "useragent")
  handler.ServeHTTP(
      kNilResponseWriter,
      request)
  expected := "192.168.5.1 - fred [23/Mar/2013:13:14:15 +0000] \"GET /foo/bar?query=tall HTTP/1.0\" 321 7 \"referer\" \"useragent\"\n"
  verifyLogs(t, expected, buf.String())
}

func TestApacheUser(t *testing.T) {
  verifyString(t, "-", weblogs.ApacheUser(nil))
  verifyString(t, "-", weblogs.ApacheUser(url.User("")))
  verifyString(t, "bill", weblogs.ApacheUser(url.User("bill")))
}

func TestAppendedLogs(t *testing.T) {
  buf := &bytes.Buffer{}
  clock := &clock{Time: kTime}
  handler := weblogs.HandlerWithOptions(
      &handler{Status: 321, LogExtra: "behere"},
      &weblogs.Options{Writer: buf, Now: clock.Now()})
  handler.ServeHTTP(
      kNilResponseWriter,
      newRequest("192.168.5.1", "GET", "/foo/bar?query=tall"))
  expected := "03/23/2013 13:14:15.123456 192.168.5.1 GET /foo/bar?query=tall 321 0 behere\n"
  verifyLogs(t, expected, buf.String())
}

func TestSend500OnNoOutput(t *testing.T) {
  buf := &bytes.Buffer{}
  clock := &clock{Time: kTime}
  handler := weblogs.HandlerWithOptions(
      &handler{LogExtra: "behere", Clock: clock, ElapsedMillis: 23},
      &weblogs.Options{Writer: buf, Now: clock.Now()})
  w := &spyResponseWriter{}
  handler.ServeHTTP(
      w,
      newRequest("192.168.5.1", "GET", "/foo/bar?query=tall"))
  expected := "03/23/2013 13:14:15.123456 192.168.5.1 GET /foo/bar?query=tall 500 23 behere\n"
  verifyLogs(t, expected, buf.String())
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

type clock struct {
  Time time.Time
}

func (c *clock) AddMillis(millis int) {
  c.Time = c.Time.Add(time.Duration(millis) * time.Millisecond)
}

func (c *clock) Now() func() time.Time {
  return func() time.Time {
    return c.Time
  }
}

func verifyLogs(t *testing.T, expected, actual string) {
  verifyString(t, expected, actual)
}

func verifyString(t *testing.T, expected, actual string) {
  if expected != actual {
    t.Errorf("Want: %s, Got: %s", expected, actual)
  }
}

func newRequest(remoteAddr, method, urlStr string) *http.Request {
  u, _ := url.Parse(urlStr)
  return &http.Request{
    RemoteAddr: remoteAddr,
    Method: method,
    Proto: "HTTP/1.0",
    URL: u}
}

type handler struct {
  Status int
  Message string
  LogExtra string
  Clock *clock
  ElapsedMillis int
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  // Misbehave by mutating request object to verify that this does not affect
  // logs
  r.URL.Path = "/HandlerMutatedRequest"
  if h.Status != 0 {
    w.WriteHeader(h.Status)
    fmt.Fprintf(w, "%s", h.Message)
  }
  if h.LogExtra != "" {
    fmt.Fprintf(weblogs.Writer(r), " %s", h.LogExtra)
  }
  if h.Clock != nil {
    h.Clock.AddMillis(h.ElapsedMillis)
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
  
