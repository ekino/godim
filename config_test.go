// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"reflect"
	"testing"
)

func configForTest(key string, val reflect.Value) (interface{}, error) {
	return nil, nil
}

func TestFullConfig(t *testing.T) {
	g := NewConfig().WithInjectString("in").WithConfigString("conf").WithConfigurationFunction(configForTest).Build()
	if g.registry.inject != "in" {
		t.Fatalf("wrong inject configuration")
	}
	if g.registry.config != "conf" {
		t.Fatalf("wrong config configuration")
	}
	if g.registry.appProfile == nil {
		t.Fatalf("AppProfile not defined")
	}
	if reflect.TypeOf(g.configFunction) != reflect.TypeOf(configForTest) {
		t.Fatalf("Wrong config function")
	}
}

func TestWrongStrings(t *testing.T) {
	g := NewConfig().WithInjectString("").WithConfigString("").Build()
	if g.registry.inject != "inject" {
		t.Fatalf("wrong inject configuration")
	}
	if g.registry.config != "config" {
		t.Fatalf("wrong config configuration")
	}
}
