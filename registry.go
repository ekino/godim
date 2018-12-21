// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	defaultInject = "inject"
	defaultConfig = "config"
)

// Registry the internal registry
type Registry struct {
	inject     string
	config     string
	appProfile *AppProfile
	values     map[string]map[string]*holder
	tags       map[reflect.Type]*TagConfig
	inits      map[reflect.Type]reflect.Value
	closers    map[reflect.Type]reflect.Value
}

type holder struct {
	o   interface{}
	typ reflect.Type
}

// TagConfig internal configuration tag
type TagConfig struct {
	configs map[string]string
	injects map[string]string
}

func newRegistry() *Registry {
	return &Registry{
		inject:     defaultInject,
		config:     defaultConfig,
		appProfile: newAppProfile(),
		values:     make(map[string]map[string]*holder),
		tags:       make(map[reflect.Type]*TagConfig),
		inits:      make(map[reflect.Type]reflect.Value),
		closers:    make(map[reflect.Type]reflect.Value),
	}
}

func newRegistryFromConfig(config *Config) *Registry {
	return &Registry{
		inject:     config.injectString,
		config:     config.configString,
		appProfile: config.appProfile,
		values:     make(map[string]map[string]*holder),
		tags:       make(map[reflect.Type]*TagConfig),
		inits:      make(map[reflect.Type]reflect.Value),
		closers:    make(map[reflect.Type]reflect.Value),
	}
}

func (registry *Registry) declare(label string, o interface{}) error {

	typ := reflect.TypeOf(o)
	if typ.Kind() == reflect.Ptr {
		// in case of a Ptr to interface
		typ = reflect.ValueOf(o).Elem().Type()
	}
	if !registry.appProfile.validate(label) {
		return newError(fmt.Errorf(" %s is not a declared profile", label)).SetErrType(ErrTypeRegistry)

	}
	v, ok := registry.values[label]
	if !ok {
		v = make(map[string]*holder)
		registry.values[label] = v
	}
	key := getKey(typ, o)
	_, ok = v[key]
	if ok {
		return newError(fmt.Errorf(" %s already defined in registry", o)).SetErrType(ErrTypeRegistry)
	}
	v[key] = &holder{o: o, typ: typ}
	err := registry.declareTags(typ, label)
	if err != nil {
		return err
	}
	return registry.declareInterfaces(o, typ)
}

var (
	initType  = reflect.TypeOf((*Initializer)(nil)).Elem()
	closeType = reflect.TypeOf((*Closer)(nil)).Elem()
	keyType   = reflect.TypeOf((*Identifier)(nil)).Elem()
)

func getKey(typ reflect.Type, o interface{}) string {
	ptyp := reflect.PtrTo(typ)
	ok := typ.Implements(keyType)
	if ptyp.Implements(keyType) || ok {
		return reflect.ValueOf(o).MethodByName("Key").Call([]reflect.Value{})[0].Interface().(string)
	}
	return typ.String()
}

func (registry *Registry) declareInterfaces(o interface{}, typ reflect.Type) error {
	_, exists := registry.inits[typ]
	if exists {
		return newError(fmt.Errorf("OnInit Method already declared for type %s", typ)).SetErrType(ErrTypeRegistry)
	}
	ptyp := reflect.PtrTo(typ)
	if ptyp.Implements(initType) {
		registry.inits[typ] = reflect.ValueOf(o).MethodByName("OnInit")
	}
	if ptyp.Implements(closeType) {
		registry.closers[typ] = reflect.ValueOf(o).MethodByName("OnClose")
	}
	return nil
}

func (registry *Registry) declareTags(typ reflect.Type, label string) error {
	tc := registry.getTagConfig(typ)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag
		ctag := tag.Get(registry.config)
		if len(ctag) > 0 {
			tc.configs[field.Name] = ctag
		}
		itag := typ.Field(i).Tag.Get(registry.inject)
		if len(itag) > 0 {
			_, err := registry.appProfile.validateTag(label, itag)
			if err != nil {
				return newError(err).SetErrType(ErrTypeRegistry)
			}
			tc.injects[typ.Field(i).Name] = itag
		}
	}
	return nil
}

func (registry *Registry) getTagConfig(typ reflect.Type) *TagConfig {
	tc := registry.tags[typ]
	if tc == nil {
		tc = &TagConfig{
			configs: make(map[string]string),
			injects: make(map[string]string),
		}
		registry.tags[typ] = tc
	}
	return tc
}

func (registry *Registry) configure(f func(key string, val reflect.Value) (interface{}, error)) error {
	for _, mv := range registry.values {
		if mv == nil {
			continue
		}
		for _, h := range mv {
			typ := h.typ
			tc := registry.tags[typ]
			if tc == nil {
				continue
			}
			elem := reflect.ValueOf(h.o).Elem()
			for fieldname, key := range tc.configs {
				err := setFieldOnValue(elem, fieldname, key, f)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func setFieldOnValue(v reflect.Value, fieldname, key string, f func(key string, val reflect.Value) (interface{}, error)) error {
	field := v.FieldByName(fieldname)
	kind := field.Kind()
	toSet, err := f(key, field)
	if err != nil {
		return err
	}
	switch kind {
	case reflect.String:
		field.SetString(toSet.(string))
	case reflect.Int64:
		field.SetInt(toSet.(int64))
	}
	return nil
}

func (registry *Registry) injection() error {
	for _, mv := range registry.values {
		if mv == nil {
			continue
		}
		for _, h := range mv {
			typ := h.typ
			tc := registry.tags[typ]
			if tc == nil {
				continue
			}
			elem := reflect.ValueOf(h.o).Elem()
			for fieldname, key := range tc.injects {
				elts := strings.Split(key, ":")
				toInject := registry.getElement(elts[0], elts[1])
				fmt.Printf("toinject : %+v \n", toInject)
				if toInject != nil {
					elem.FieldByName(fieldname).Set(reflect.ValueOf(toInject))
				}

			}
		}
	}
	return nil
}

func (registry *Registry) getElement(label, key string) interface{} {
	m := registry.values[label]
	if m == nil {
		return nil
	}
	for k, h := range m {
		if key == k {
			return h.o
		}
	}
	return nil
}

func (registry *Registry) initializeAll() error {
	for _, m := range registry.inits {
		ret := m.Call([]reflect.Value{})
		if len(ret) > 0 {
			err := ret[0]
			if !err.IsNil() {
				return err.Interface().(error)
			}
		}
	}
	return nil
}

func (registry *Registry) closeAll() error {
	for _, m := range registry.closers {
		ret := m.Call([]reflect.Value{})
		if len(ret) > 0 {
			err := ret[0]
			if !err.IsNil() {
				return err.Interface().(error)
			}
		}
	}
	return nil
}
