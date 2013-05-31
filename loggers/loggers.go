// Copyright 2013 Travis Keep. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or
// at http://opensource.org/licenses/BSD-3-Clause.

// Package loggers provides routines for creating weblogs.Logger
// implementations.
package loggers

import (
  "net/http"
  "net/url"
  "strings"
)

// Snapshot provides a basic snapshot of a request.
type Snapshot struct {
  // Copied from Request.RemoteAddr
  RemoteAddr string
  // Copied from Request.Method
  Method string
  // Copied from Request.Proto
  Proto string
  // Copied from Request.URL
  URL *url.URL
  // The http referer
  Referer string
  // The user agent
  UserAgent string
}

func NewSnapshot(r *http.Request) *Snapshot {
  urlSnapshot := *r.URL
  return &Snapshot{
      RemoteAddr: r.RemoteAddr,
      Method: r.Method,
      Proto: r.Proto,
      URL: &urlSnapshot,
      Referer: r.Referer(),
      UserAgent: r.UserAgent()}
}

// Capture provides a basic capture of a response
type Capture struct {
  // The underlying ResponseWriter
  http.ResponseWriter
  status int
  size int
  statusSet bool
}

// The HTTP status code of the response
func (c *Capture) Status() int {
  return c.status
}

// The size of the resposne
func (c *Capture) Size() int {
  return c.size
}

func (c *Capture) Write(b []byte) (int, error) {
  result, err := c.ResponseWriter.Write(b)
  c.size += result
  c.maybeSetStatus(http.StatusOK)
  return result, err
}

func (c *Capture) WriteHeader(status int) {
  c.ResponseWriter.WriteHeader(status)
  c.maybeSetStatus(status)
}

// HasStatus returns true if response has a status.
func (c *Capture) HasStatus() bool {
  return c.statusSet
}

func (c *Capture) maybeSetStatus(status int) {
  if !c.statusSet {
    c.status = status
    c.statusSet = true
  }
}

// ApacheUser formats user info in a request in apache style. That is missing
// or empty user name is formatted as '-'
func ApacheUser(user *url.Userinfo) string {
  result := "-"
  if user != nil {
    if name := user.Username(); name != "" {
      result = name
    }
  }
  return result
}

// StripPort strips the port number off of a remote address
func StripPort(remoteAddr string) string {
  if index := strings.LastIndex(remoteAddr, ":"); index != -1 {
    return remoteAddr[:index]
  }
  return remoteAddr
}
