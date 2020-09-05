// Copyright 2013 Travis Keep. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or
// at http://opensource.org/licenses/BSD-3-Clause.

package loggers_test

import (
	"github.com/keep94/weblogs/loggers"
	"net/url"
	"testing"
)

func TestApacheUser(t *testing.T) {
	verifyString(t, "-", loggers.ApacheUser(nil))
	verifyString(t, "-", loggers.ApacheUser(url.User("")))
	verifyString(t, "tom", loggers.ApacheUser(url.User("tom")))
}

func TestStripPort(t *testing.T) {
	verifyString(t, "[::1]", loggers.StripPort("[::1]:4050"))
	verifyString(t, "10.0.1.3", loggers.StripPort("10.0.1.3:25972"))
	verifyString(t, "10.0.1.3", loggers.StripPort("10.0.1.3"))
}

func verifyString(t *testing.T, expected, actual string) {
	if expected != actual {
		t.Errorf("Want: %s, Got: %s", expected, actual)
	}
}
