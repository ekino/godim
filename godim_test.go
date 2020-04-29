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

const (
	ServiceLabel = "service"
	BKey         = "bbb"
	Yes          = "yes"
	No           = "no"
)

type Foo struct{}
type Bar struct{}

func config(key string) interface{} {
	fmt.Printf("Searching for key %s \n", key)
	return key
}

func TestGodim_DeclareDefault(t *testing.T) {
	g := Default()

	foo := Foo{}
	bar := Bar{}

	err := g.DeclareDefault(foo, bar)
	if err != nil {
		t.Fatalf("Error while declaring default: %s.", err)
	}

	err = g.DeclareDefault(&Foo{})
	if err == nil {
		t.Fatal("An error is expected.")
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
	if a.B.Initialized == Yes {
		a.PreInitialized = Yes
	} else {
		a.PreInitialized = No
	}
	return nil
}

type B struct {
	Initialized string
}

func (b *B) Key() string {
	return BKey
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

func TestGodim_Declare_shouldHandleDependenciesAndGetThemBack(t *testing.T) {
	g := NewConfig().WithAppProfile(StrictHTTPAppProfile()).WithConfigurationFunction(conf).Build()

	a := A{}
	b := B{}
	c := C{}

	err := g.Declare(ServiceLabel, &b)
	if err != nil {
		t.Fatalf("Error while declaring 'service': %s.", err)
	}

	err = g.Declare("handler", &a, &c)
	if err != nil {
		t.Fatalf("Error while declaring 'handler': %s.", err)
	}

	err = g.RunApp()

	ob := g.GetStruct(ServiceLabel, BKey)

	if ob == nil {
		t.Fatal("Should have retrieved service B")
	}

	if err != nil {
		t.Fatalf("Error while configuring %s.", err)
	}

	if c.Val != 42 {
		t.Fatalf("OnInit initialization failed. Expecting 42 but got %d.", c.Val)
	}

	if a.Lab != "bid" {
		t.Fatalf("string property got wrong value: 'bid' expected but got '%s'.", a.Lab)
	}

	if c.Myint != 12 {
		t.Fatalf("int64 property got wrong value: 12 expected but got %d.", c.Myint)
	}

	if a.B != &b {
		t.Fatalf("A got an unexpected value for B: got %+v but expected %+v.", a.B, &b)
	}
	if c.B != &b {
		t.Fatalf("C got an unexpected value for B: got %+v but expected %+v.", c.B, &b)
	}

	if !g.lifecycle.current(stRun) {
		t.Fatalf("Wrong state: %s.", g.lifecycle)
	}

	err = g.CloseApp()
	if err != nil {
		t.Fatal("Error while closing app")
	}

	if !g.lifecycle.current(stClose) {
		t.Fatalf("Wrong state: %s.", g.lifecycle)
	}

	if c.Val != 24 {
		t.Fatalf("OnClose finalization failed. Expecting 24 but got %d.", c.Val)
	}

	// Check initialization priorities
	if b.Initialized != Yes {
		t.Fatal("b OnInit not called")
	}

	if a.PreInitialized == Yes {
		t.Fatal("a priority not respected in regard to b")
	}

	if c.PreInitialized != Yes {
		t.Fatal("b Priority not respected")
	}
}

func TestGodim_RunApp_shouldRunTheEventSwitchWhenConfigured(t *testing.T) {
	g := NewConfig().WithAppProfile(StrictHTTPAppProfile()).
		WithConfigurationFunction(conf).
		WithEventSwitch(10).
		Build()

	emitter := new(SimpleEmitter)
	receiver := newCounterReceiver(1)

	err := g.Declare("service", emitter, receiver)
	if err != nil {
		t.Fatal("Error while declaring service:", err)
	}

	err = g.RunApp()

	for i := 0; i < 100; i++ {
		emitter.Emit(newEmptyEvent(EventTypeA))
		time.Sleep(1 * time.Millisecond)
	}

	time.Sleep(5 * time.Millisecond)
	if receiver.receivedEventCount != 100 {
		t.Fatalf("Wrong number of event received. 100 expected, but got %d.", receiver.receivedEventCount)
	}

	g.CloseApp()

	time.Sleep(1 * time.Millisecond)
	if g.eventSwitch.running {
		t.Fatal("EventSwitch should be closed.")
	}
}

func TestGodim_CloseAppGracefully_shouldGracefullyCloseTheEventSwitch(t *testing.T) {
	g := NewConfig().WithAppProfile(StrictHTTPAppProfile()).
		WithConfigurationFunction(conf).
		WithEventSwitch(10).
		Build()

	emitter := new(SimpleEmitter)
	receiver := newCounterReceiver(1)
	interceptor := new(DelayInterceptor)

	err := g.Declare("service", emitter, receiver, interceptor)
	if err != nil {
		t.Fatal("Error while declaring service:", err)
	}

	err = g.RunApp()

	for i := 0; i < 100; i++ {
		emitter.Emit(newEmptyEvent(EventTypeA))
	}
	go g.CloseAppGracefully()

	time.Sleep(100 * time.Millisecond)

	if !g.lifecycle.current(stRun) {
		t.Fatal("The graceful closing of the application should shutdown the EventEmitter first.")
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
