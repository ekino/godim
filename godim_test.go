// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"fmt"
	"reflect"
	"testing"
)

type Bidule struct{}
type Truc struct{}

func config(key string) interface{} {
	fmt.Printf("Searching for key %s \n", key)
	return key
}

func TestDefaultGodim(t *testing.T) {
	g := Default()
	a := Bidule{}
	b := Truc{}
	err := g.DeclareDefault(a, b)
	if err != nil {
		t.Fatalf("Error while declaring default %s \n", err)
	}
	err = g.DeclareDefault(&Bidule{})
	if err == nil {
		t.Fatalf("there must be an error")
	}
}

type A struct {
	B   *B     `inject:"service:godim.B"`
	Lab string `config:"lab.key"`
}

type B struct {
}

type C struct {
	B     *B    `inject:"service:godim.B"`
	Myint int64 `config:"myint.key"`
	Val   int
}

func (c *C) OnInit() error {
	c.Val = 42
	return nil
}

func (c *C) OnClose() error {
	c.Val = 24
	return nil
}

func TestDeclare(t *testing.T) {
	g := NewConfig().WithAppProfile(StrictHTTPAppProfile()).WithConfigurationFunction(conf).Build()
	a := A{}
	b := B{}
	c := C{}
	err := g.Declare("service", &b)
	if err != nil {
		t.Fatalf("error while declaring service %s \n", err)
	}
	err = g.Declare("handler", &a, &c)
	if err != nil {
		t.Fatalf("error while declaring handlers %s \n", err)
	}
	fmt.Println("configuration phase")
	err = g.RunApp()
	if err != nil {
		t.Fatalf(" error while configuring %s \n", err)
	}
	if a.Lab != "bid" {
		t.Fatalf("misconfig on string")
	}
	if c.Myint != 12 {
		t.Fatalf("misconfig on int64")
	}

	fmt.Printf("A : %+v\n", a)
	if a.B != &b || c.B != &b {
		t.Fatalf("misinjection %+v %+v", a.B, &b)
	}
	if !g.lifecycle.current(stRun) {
		t.Fatalf("Wrong state %s", g.lifecycle)
	}
	if c.Val != 42 {
		t.Fatalf("Wrong initialization OnInit")
	}
	err = g.CloseApp()
	if err != nil {
		t.Fatalf("Error while closing app")
	}

	if !g.lifecycle.current(stClose) {
		t.Fatalf("Wrong state %s", g.lifecycle)
	}
	if c.Val != 24 {
		t.Fatalf("no Call to OnClose")
	}
}

func conf(key string, val reflect.Value) (interface{}, error) {

	if key == "myint.key" {
		var i int64
		i = 12
		return i, nil
	}
	if key == "lab.key" {
		return "bid", nil
	}
	return nil, fmt.Errorf("unknow key %s", key)
}
