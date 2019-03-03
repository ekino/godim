// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

const (
	defaultInject   = "inject"
	defaultConfig   = "config"
	defaultPriority = 0
)

// Registry the internal registry
type Registry struct {
	inject      string
	config      string
	appProfile  *AppProfile
	values      map[string]map[string]*holder
	tags        map[reflect.Type]*TagConfig
	inits       map[int]map[reflect.Type]reflect.Value
	closers     map[reflect.Type]reflect.Value
	eventSwitch *EventSwitch
}

type holder struct {
	o    interface{}
	typ  reflect.Type
	prio int
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
		inits:      make(map[int]map[reflect.Type]reflect.Value),
		closers:    make(map[reflect.Type]reflect.Value),
	}
}

func newRegistryFromConfig(config *Config) *Registry {
	r := &Registry{
		inject:     config.injectString,
		config:     config.configString,
		appProfile: config.appProfile,
		values:     make(map[string]map[string]*holder),
		tags:       make(map[reflect.Type]*TagConfig),
		inits:      make(map[int]map[reflect.Type]reflect.Value),
		closers:    make(map[reflect.Type]reflect.Value),
	}
	if config.activateES {
		r.eventSwitch = config.eventSwitch
	}
	return r
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
	prio := getPriority(typ, o)
	_, ok = v[key]
	if ok {
		return newError(fmt.Errorf(" %s already defined in registry", o)).SetErrType(ErrTypeRegistry)
	}
	v[key] = &holder{o: o, typ: typ, prio: prio}
	err := registry.declareTags(typ, label)
	if err != nil {
		return err
	}
	return registry.declareInterfaces(o, typ)
}

var (
	initType      = reflect.TypeOf((*Initializer)(nil)).Elem()
	closeType     = reflect.TypeOf((*Closer)(nil)).Elem()
	keyType       = reflect.TypeOf((*Identifier)(nil)).Elem()
	prioType      = reflect.TypeOf((*Prioritizer)(nil)).Elem()
	emitType      = reflect.TypeOf((*Emitter)(nil)).Elem()
	recType       = reflect.TypeOf((*EventReceiver)(nil)).Elem()
	interceptType = reflect.TypeOf((*EventInterceptor)(nil)).Elem()
	finalizerType = reflect.TypeOf((*EventFinalizer)(nil)).Elem()
)

func getKey(typ reflect.Type, o interface{}) string {
	ptyp := reflect.PtrTo(typ)
	ok := typ.Implements(keyType)
	if ptyp.Implements(keyType) || ok {
		return reflect.ValueOf(o).MethodByName("Key").Call([]reflect.Value{})[0].Interface().(string)
	}
	return strings.Split(typ.String(), ".")[1]
}

func getPriority(typ reflect.Type, o interface{}) int {
	ptyp := reflect.PtrTo(typ)
	ok := typ.Implements(prioType)
	if ptyp.Implements(prioType) || ok {
		return reflect.ValueOf(o).MethodByName("Priority").Call([]reflect.Value{})[0].Interface().(int)
	}
	return defaultPriority
}

func (registry *Registry) declareInterfaces(o interface{}, typ reflect.Type) error {
	prio := getPriority(typ, o)
	_, exists := registry.inits[prio][typ]
	if exists {
		return newError(fmt.Errorf("OnInit Method already declared for type %s", typ)).SetErrType(ErrTypeRegistry)
	}
	ptyp := reflect.PtrTo(typ)
	if ptyp.Implements(initType) {
		v, ok := registry.inits[prio]
		if !ok {
			v = make(map[reflect.Type]reflect.Value)
			registry.inits[prio] = v
		}
		registry.inits[prio][typ] = reflect.ValueOf(o).MethodByName("OnInit")
	}
	if ptyp.Implements(closeType) {
		registry.closers[typ] = reflect.ValueOf(o).MethodByName("OnClose")
	}
	if registry.eventSwitch != nil {
		if ptyp.Implements(emitType) {
			registry.eventSwitch.AddEmitter(o.(Emitter))
		}
		if ptyp.Implements(recType) {
			registry.eventSwitch.AddReceiver(o.(EventReceiver))
		}
		if ptyp.Implements(interceptType) {
			registry.eventSwitch.AddInterceptor(o.(EventInterceptor))
		}
		if ptyp.Implements(finalizerType) {

		}
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
	toSet, err := f(key, field)
	if err != nil {
		return err
	}
	field.Set(reflect.ValueOf(toSet))
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
	// Sort by priority
	priorities := make([]int, len(registry.inits))
	i := 0
	for k := range registry.inits {
		priorities[i] = k
		i++
	}
	sort.Ints(priorities)

	for _, p := range priorities {
		for _, m := range registry.inits[p] {
			ret := m.Call([]reflect.Value{})
			if len(ret) > 0 {
				err := ret[0]
				if !err.IsNil() {
					return err.Interface().(error)
				}
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
