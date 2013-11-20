// Copyright 2013 Travis Keep. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or
// at http://opensource.org/licenses/BSD-3-Clause.

package weblogs_test

import (
  "bytes"
  "fmt"
  "github.com/keep94/weblogs"
  "github.com/keep94/weblogs/loggers"
  "io"
  "net/http"
  "net/url"
  "sync"
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
      newRequest("192.168.5.1:3333", "GET", "/foo/bar?query=tall"))
  expected := "03/23/2013 13:14:15 192.168.5.1 GET /foo/bar?query=tall 321 387\n"
  verifyLogs(t, expected, buf.String())
}

func TestRaces(t *testing.T) {
  buf := &bytes.Buffer{}
  clock := &clock{Time: kTime}
  handler := weblogs.HandlerWithOptions(
      &handler{Status: 200},
      &weblogs.Options{
          Writer: buf, Logger: verySimpleLogger{}, Now: clock.Now()})
  var wg sync.WaitGroup
  wg.Add(20)
  for i := 0; i < 20; i++ {
    go func() {
      handler.ServeHTTP(
          kNilResponseWriter,
          newRequest("192.168.5.1:3333", "GET", "/foo/bar?query=tall"))
      wg.Done()
    }()
  }
  wg.Wait()
}

func TestCommonLogs(t *testing.T) {
  buf := &bytes.Buffer{}
  clock := &clock{Time: kTime}
  handler := weblogs.HandlerWithOptions(
      &handler{Status: 321, Message: "1234567"},
      &weblogs.Options{
          Writer: buf,
          Logger: weblogs.ApacheCommonLogger(),
          Now: clock.Now()})
  request := newRequest("192.168.5.1:3333", "GET", "/foo/bar?query=tall")
  request.URL.User = url.User("fred")
  handler.ServeHTTP(
      kNilResponseWriter,
      request)
  expected := "192.168.5.1 - fred [23/Mar/2013:13:14:15 +0000] \"GET /foo/bar?query=tall HTTP/1.0\" 321 7\n"
  verifyLogs(t, expected, buf.String())
}

func TestLogStatus200(t *testing.T) {
  buf := &bytes.Buffer{}
  clock := &clock{Time: kTime}
  handler := weblogs.HandlerWithOptions(
      &handler{Message: "1234567"},
      &weblogs.Options{
          Writer: buf,
          Logger: weblogs.ApacheCommonLogger(),
          Now: clock.Now()})
  request := newRequest("192.168.5.1:3333", "GET", "/foo/bar?query=tall")
  request.URL.User = url.User("fred")
  handler.ServeHTTP(
      kNilResponseWriter,
      request)
  expected := "192.168.5.1 - fred [23/Mar/2013:13:14:15 +0000] \"GET /foo/bar?query=tall HTTP/1.0\" 200 7\n"
  verifyLogs(t, expected, buf.String())
}

func TestCombinedLogs(t *testing.T) {
  buf := &bytes.Buffer{}
  clock := &clock{Time: kTime}
  handler := weblogs.HandlerWithOptions(
      &handler{Status: 321, Message: "1234567"},
      &weblogs.Options{
          Writer: buf,
          Logger: weblogs.ApacheCombinedLogger(),
          Now: clock.Now()})
  request := newRequest("192.168.5.1:3333", "GET", "/foo/bar?query=tall")
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

func TestAppendedLogs(t *testing.T) {
  buf := &bytes.Buffer{}
  clock := &clock{Time: kTime}
  handler := weblogs.HandlerWithOptions(
      &handler{Status: 321, LogExtra: "behere"},
      &weblogs.Options{Writer: buf, Now: clock.Now()})
  handler.ServeHTTP(
      kNilResponseWriter,
      newRequest("192.168.5.1", "GET", "/foo/bar?query=tall"))
  expected := "03/23/2013 13:14:15 192.168.5.1 GET /foo/bar?query=tall 321 0 behere\n"
  verifyLogs(t, expected, buf.String())
}

func TestSetValues(t *testing.T) {
  buf := &bytes.Buffer{}
  handler := weblogs.HandlerWithOptions(
      &handler{Field1: "one", Field2: "two"},
      &weblogs.Options{Writer: buf, Logger: setLogger{}})
  handler.ServeHTTP(
      kNilResponseWriter,
      newRequest("192.168.5.1", "GET", "/foo/bar?query=tall"))
  expected := "field1=one field2=two\n"
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
      newRequest("192.168.5.1:3333", "GET", "/foo/bar?query=tall"))
  expected := "03/23/2013 13:14:15 192.168.5.1 GET /foo/bar?query=tall 500 23 behere\n"
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
      newRequest("192.168.5.1:3333", "GET", "/foo/bar?query=tall"))
}

func TestUnwrappedCallToValues(t *testing.T) {
  if weblogs.Values(newRequest(
      "192.168.5.1:3333", "GET", "/foo/bar?query=tall")) != nil {
    t.Error("Expected unwrapped weblogs.Values call to return nil")
  }
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
  Field1 string
  Field2 string
  Clock *clock
  ElapsedMillis int
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  // Misbehave by mutating request object to verify that this does not affect
  // logs
  r.URL.Path = "/HandlerMutatedRequest"
  if h.Status != 0 {
    w.WriteHeader(h.Status)
  }
  if h.Message != "" {
    fmt.Fprintf(w, "%s", h.Message)
  }
  if h.LogExtra != "" {
    fmt.Fprintf(weblogs.Writer(r), " %s", h.LogExtra)
  }
  values := weblogs.Values(r)
  if values != nil {
    if h.Field1 != "" {
      values["field1"] = h.Field1
    }
    if h.Field2 != "" {
      values["field2"] = h.Field2
    }
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

type setLogger struct {
}

func (s setLogger) NewSnapshot(r *http.Request) weblogs.Snapshot {
  return nil
}

func (s setLogger) NewCapture(w http.ResponseWriter) weblogs.Capture {
  return &loggers.Capture{ResponseWriter: w}
}

func (s setLogger) Log(w io.Writer, record *weblogs.LogRecord) {
  fmt.Fprintf(
      w,
      "field1=%v field2=%v\n",
      record.Values["field1"],
      record.Values["field2"])
}

type verySimpleLogger struct {
}

func (l verySimpleLogger) NewSnapshot(r *http.Request) weblogs.Snapshot {
  return nil
}

func (l verySimpleLogger) NewCapture(
    w http.ResponseWriter) weblogs.Capture {
  return &loggers.Capture{ResponseWriter: w}
}

func (l verySimpleLogger) Log(w io.Writer, record *weblogs.LogRecord) {
  w.Write([]byte{72,69,76,76,79})
}



