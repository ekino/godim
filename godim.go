// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"fmt"
	"reflect"
)

// Godim is the main app controller
type Godim struct {
	lifecycle      *lifecycle
	registry       *Registry
	configFunction func(key string, val reflect.Value) (interface{}, error)
}

// Default build a default godim from default configuration
func Default() *Godim {
	return DefaultConfig().Build()
}

// NewGodim build a Godim with specified Config
func NewGodim(config *Config) *Godim {
	var g Godim
	g.lifecycle = newLifecycle()
	g.registry = newRegistryFromConfig(config)
	g.configFunction = config.configFunction
	return &g
}

// DeclareDefault : declare all your defaults services
func (godim *Godim) DeclareDefault(o ...interface{}) error {
	if godim.lifecycle.current(stDeclaration) {
		for _, v := range o {
			err := godim.registry.declare(defaultStr, v)
			if err != nil {
				return newError(err).SetErrType(ErrTypeGodim)

			}
		}
	} else {
		return newError(fmt.Errorf("Current phase %s", godim.lifecycle)).SetErrType(ErrTypeGodim)
	}
	return nil
}

// Declare specific level
func (godim *Godim) Declare(label string, o ...interface{}) error {
	if godim.lifecycle.current(stDeclaration) {
		for _, v := range o {
			err := godim.registry.declare(label, v)
			if err != nil {
				return newError(err).SetErrType(ErrTypeGodim)
			}
		}
	} else {
		return newError(fmt.Errorf("Current phase %s", godim.lifecycle)).SetErrType(ErrTypeGodim)
	}
	return nil
}

func (godim *Godim) configure() error {
	if godim.lifecycle.current(stDeclaration) {
		godim.lifecycle.currentState++
		if godim.configFunction != nil {
			err := godim.registry.configure(godim.configFunction)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (godim *Godim) injection() error {
	if godim.lifecycle.current(stConfiguration) {
		godim.lifecycle.currentState++
		err := godim.registry.injection()
		if err != nil {
			return err
		}
	}
	return nil
}

func (godim *Godim) initialize() error {
	if godim.lifecycle.current(stInjection) {
		godim.lifecycle.currentState++
		err := godim.registry.initializeAll()
		if err != nil {
			return err
		}
	}
	return nil
}

//
// RunApp : Run the application after configuration and injection phase
//
func (godim *Godim) RunApp() error {
	if godim.lifecycle.current(stRun) || godim.lifecycle.current(stClose) {
		return newError(fmt.Errorf("Godim is already in state %s", godim.lifecycle)).SetErrType(ErrTypeGodim)
	}
	// Configuration phase
	err := godim.configure()
	if err != nil {
		return err
	}
	// Injection phase
	err = godim.injection()
	if err != nil {
		return err
	}
	// Initializer phase
	err = godim.initialize()
	if err != nil {
		return err
	}
	// Run phase
	if godim.lifecycle.current(stInitialization) {
		godim.lifecycle.currentState++
	}
	return nil
}

// CloseApp close all things declared in your app
func (godim *Godim) CloseApp() error {
	if godim.lifecycle.current(stRun) {
		err := godim.registry.closeAll()
		if err != nil {
			return err
		}
		godim.lifecycle.currentState++
	}
	return nil
}

// GetStruct return the stored struct in case it is needed for other usage
func (godim *Godim) GetStruct(label, key string) interface{} {
	return godim.registry.getElement(label, key)
}
