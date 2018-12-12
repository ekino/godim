// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import "testing"

func TestFullConfig(t *testing.T) {
	g := NewConfig().WithInjectString("in").WithConfigString("conf").Build()
	if g.registry.inject != "in" {
		t.Fatalf("wrong inject configuration")
	}
	if g.registry.config != "conf" {
		t.Fatalf("wrong config configuration")
	}
	if g.registry.appProfile == nil {
		t.Fatalf("AppProfile not defined")
	}
}
