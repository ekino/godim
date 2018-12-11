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
	configFunction func(key string, kind reflect.Kind) (interface{}, error)
}

// NewConfig declare a new config
func NewConfig() *Config {
	return &Config{
		injectString: defaultInject,
		configString: defaultConfig,
	}
}

// DefaultConfig declare a default configuration
func DefaultConfig() *Config {
	return &Config{
		injectString: defaultInject,
		configString: defaultConfig,
		appProfile:   newAppProfile(),
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
func (c *Config) WithConfigurationFunction(f func(key string, kind reflect.Kind) (interface{}, error)) *Config {
	c.configFunction = f
	return c
}

// Build lock profile and build godim
func (c *Config) Build() *Godim {
	if c.appProfile == nil {
		c.appProfile = newAppProfile()
	}
	c.appProfile.lock()
	return NewGodim(c)
}
