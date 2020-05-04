// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"log"
	"reflect"
	"strings"
)

// Config struct for Godim
type Config struct {
	injectString   string
	configString   string
	appProfile     *AppProfile
	configFunction func(key string, val reflect.Value) (interface{}, error)
	activateES     bool
	bufferSize     int
	eventSwitch    *EventSwitch
}

// NewConfig declare a new config
func NewConfig() *Config {
	return &Config{
		injectString: defaultInject,
		configString: defaultConfig,
		activateES:   false,
	}
}

// DefaultConfig declare a default configuration
func DefaultConfig() *Config {
	return &Config{
		injectString: defaultInject,
		configString: defaultConfig,
		appProfile:   NewAppProfile(),
		activateES:   false,
	}
}

// WithInjectString use a new inject string
func (c *Config) WithInjectString(inject string) *Config {
	i := strings.TrimSpace(inject)
	if len(i) > 0 {
		c.injectString = i
	} else {
		log.Printf("Inject string %s ignored", inject)
	}
	return c
}

// WithConfigString use a new config string
func (c *Config) WithConfigString(config string) *Config {
	i := strings.TrimSpace(config)
	if len(i) > 0 {
		c.configString = i
	} else {
		log.Printf("Config string %s ignored", config)
	}
	return c
}

// WithAppProfile declare the app profile to use
func (c *Config) WithAppProfile(ap *AppProfile) *Config {
	if ap != nil {
		c.appProfile = ap
	}
	return c
}

// WithConfigurationFunction declare your configuration function
func (c *Config) WithConfigurationFunction(f func(key string, val reflect.Value) (interface{}, error)) *Config {
	c.configFunction = f
	return c
}

// WithEventSwitch start an event switch with godim
func (c *Config) WithEventSwitch(bufferSize int) *Config {
	c.activateES = true
	c.bufferSize = bufferSize
	return c
}

// Build lock profile and build godim
func (c *Config) Build() *Godim {
	if c.appProfile == nil {
		c.appProfile = NewAppProfile()
	}
	c.appProfile.lock()
	if c.activateES {
		c.eventSwitch = NewEventSwitch(c.bufferSize)
	}
	return NewGodim(c)
}
