// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"fmt"
	"reflect"
	"testing"
)

type myStruct struct {
	a string `config:"truc"`
}

func (m *myStruct) Key() string {
	return "testkey"
}

func TestDefault(t *testing.T) {
	r := newRegistry()
	r.appProfile.lock()

	m := myStruct{
		a: "a",
	}

	err := r.declare("default", &m)
	if err != nil {
		t.Fatalf("nothing added")
	}
	if !r.appProfile.isLocked() {
		t.Fatalf("profiles not locked")
	}
	typ := reflect.TypeOf(m)
	n := r.getElement("default", "testkey")
	if n == nil {
		t.Fatalf("Can't find declared value")
	}
	if n != &m {
		t.Fatalf("not the same")
	}
	tc := r.tags[typ]
	if tc == nil {
		t.Fatalf("tag not declared")
	}
	if tc.configs["a"] != "truc" {
		t.Fatalf("wrong tag reading")
	}
}

type MyService struct {
	OtherService *MyService `inject:"service:godim.MyService"`
}

type OneHandler struct {
	Another *MyService `inject:"service:godim.MyService"`
}

func TestMultiProfile(t *testing.T) {
	r := newRegistry()
	r.appProfile = StrictHTTPAppProfile()

	b := &MyService{}

	m := MyService{
		OtherService: b,
	}
	err := r.declare("service", m)
	if err != nil {
		c := OneHandler{}
		err = r.declare("handler", c)
		if err != nil {
			t.Fatalf("Injection possible")
		}
	} else {
		t.Fatalf("There must be an error")
	}
}

func TestPtr(t *testing.T) {
	r := newRegistry()
	r.appProfile = StrictHTTPAppProfile()

	a := &OneHandler{}
	err := r.declare("thing", a)
	if err == nil {
		t.Fatal("thing doesn't exists")
	}
	err = r.declare("handler", a)
	if err != nil {
		t.Fatal("err ", err)
	} else {
		b := OneHandler{}
		err = r.declare("handler", b)
		if err == nil {
			t.Fatalf("There must be an already declared error")
		}
	}
}

type TestConfig struct {
	a string `config:"cle"`
}

func TestTags(t *testing.T) {
	r := newRegistry()
	r.appProfile = StrictHTTPAppProfile()
	typ := reflect.TypeOf(TestConfig{})
	err := r.declareTags(typ, "service")
	if err != nil {
		t.Fatalf("error while declaring tags")
	}
	tc := r.tags[typ]
	if tc == nil {
		t.Fatalf("no tagconfig")
	}
	if tc.configs["a"] != "cle" {
		fmt.Printf("%+v\n", tc.configs)
		t.Fatalf("wrong tag reading")
	}
}

type NotIniter struct {
}

type Initer struct {
	Initializer
}

func (i *Initer) OnInit() error {
	return nil
}

func TestInterface(t *testing.T) {
	r := newRegistry()
	r.appProfile = StrictHTTPAppProfile()
	ni := NotIniter{}
	not := reflect.TypeOf(ni)
	err := r.declareInterfaces(&ni, not)
	if err != nil {
		t.Fatalf("laze")
	}
	i := Initer{}
	ini := reflect.TypeOf(i)
	err = r.declareInterfaces(&i, ini)
	if err != nil {
		t.Fatalf("laze2 %s", err)
	}
}

func TestGetKey(t *testing.T) {
	tc := &TestConfig{}
	typ := reflect.TypeOf(tc).Elem()
	key := getKey(typ, tc)
	if key != "godim.TestConfig" {
		t.Fatalf("Wrong key retrieval %s", key)
	}

	ms := &myStruct{}
	typ = reflect.TypeOf(ms).Elem()
	key = getKey(typ, ms)
	if key != "testkey" {
		t.Fatalf("Wrong key retrieval %s from method", key)
	}

}
