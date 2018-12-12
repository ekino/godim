// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"testing"
)

const (
	h = "handler"
	s = "service"
	r = "repository"
)

func TestNewHTTPStrict(t *testing.T) {
	ap := StrictHTTPAppProfile()
	if len(ap.profiles) != 3 {
		t.Fatalf("wrong number of profiles")
	}
	if ap.profiles[h].canBeInjectedIn(s) {
		t.Fatalf("handler can't be injected in services")
	}
	if ap.profiles[h].canBeInjectedIn(r) {
		t.Fatalf("handler can't be injected in repositories")
	}
	if ap.profiles[h].canBeInjectedIn(h) {
		t.Fatalf("handler can't be injected in handlers")
	}
	if !ap.profiles[s].canBeInjectedIn(h) {
		t.Fatalf("service must be injectable in handlers")
	}
	if ap.profiles[s].canBeInjectedIn(s) {
		t.Fatalf("service can't be injected in services")
	}
	if ap.profiles[s].canBeInjectedIn(r) {
		t.Fatalf("service can't be injected in repositories")
	}
	if !ap.profiles[r].canBeInjectedIn(s) {
		t.Fatalf("repository must be injectable in services")
	}
	if ap.profiles[r].canBeInjectedIn(h) {
		t.Fatalf("repository can't be injected in handlers")
	}
	if ap.profiles[r].canBeInjectedIn(r) {
		t.Fatalf("repository can't be injected in repositories")
	}
	if !ap.isLocked() {
		t.Fatalf("strict profile must be locked")
	}
}

func TestHttpProfile(t *testing.T) {
	ap := HTTPAppProfile()
	if len(ap.profiles) != 3 {
		t.Fatalf("wrong number of profiles")
	}
	if !ap.profiles[s].canBeInjectedIn(s) {
		t.Fatalf("services should be injected in services")
	}
}
