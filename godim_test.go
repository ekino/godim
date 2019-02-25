// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"fmt"
	"reflect"
	"testing"
	"time"
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
	B              *B     `inject:"service:bbb"`
	Lab            string `config:"lab.key"`
	PreInitialized string
}

func (a *A) Priority() int {
	return -10
}

func (a *A) OnInit() error {
	if a.B.Initialized == "yes" {
		a.PreInitialized = "yes"
	} else {
		a.PreInitialized = "no"
	}
	return nil
}

type B struct {
	Initialized string
}

func (b *B) Key() string {
	return "bbb"
}

func (b *B) OnInit() error {
	b.Initialized = "yes"
	return nil
}

func (b *B) Priority() int {
	return -1
}

type C struct {
	B              *B    `inject:"service:bbb"`
	Myint          int64 `config:"myint.key"`
	Val            int
	PreInitialized string
}

func (c *C) OnInit() error {
	c.Val = 42
	c.PreInitialized = c.B.Initialized
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

	ob := g.GetStruct("service", "bbb")

	if ob == nil {
		t.Fatalf("must have retrieve service b")
	}

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

	// Check initialization priorities
	if b.Initialized != "yes" {
		t.Fatalf("b OnInit not called")
	}

	if a.PreInitialized == "yes" {
		t.Fatalf("a priority not respected in regard to b")
	}

	if c.PreInitialized != "yes" {
		t.Fatalf("b Priority not respected")
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

type EA struct {
	EventEmitter
}

type RA struct {
	nbReceived int
}

func (ra *RA) OnInit() error {
	ra.nbReceived = 0
	return nil
}

func (ra *RA) HandleEventTypes() []string {
	return []string{
		"aa",
	}
}
func (ra *RA) ReceiveEvent(e *Event) {
	ra.nbReceived = ra.nbReceived + 1
}

func TestWithEventSwitch(t *testing.T) {
	g := NewConfig().WithAppProfile(StrictHTTPAppProfile()).
		WithConfigurationFunction(conf).
		WithEventSwitch(10).
		Build()
	ea := new(EA)
	ra := new(RA)
	err := g.Declare("service", ea, ra)
	if err != nil {
		t.Fatal("Error while declaring service:", err)
	}

	err = g.RunApp()

	for i := 0; i < 100; i++ {
		e := &Event{
			Type: "aa",
		}
		ea.Emit(e)
		time.Sleep(1 * time.Millisecond)
	}

	time.Sleep(5 * time.Millisecond)
	if ra.nbReceived != 100 {
		t.Fatal("wrong number of event received", ra.nbReceived)
	}
	g.CloseApp()
	time.Sleep(1 * time.Millisecond)
	if g.eventSwitch.running {
		t.Fatal("eventswitch not closed")
	}
}
