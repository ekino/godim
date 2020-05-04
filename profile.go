// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"fmt"
	"strings"
)

// AppProfile represents the differents Profile of your application
// see @Profile for Profile detail
type AppProfile struct {
	profiles map[string]*Profile
	locked   bool
}

// Profile a Profile represent an injectable type
//
// It holds the possible dependency tree in order to validate non-cycling app
//
// injection example :
// type UserService struct {
//     userRepository *UserRepository inject:"repository:UserRepository"
// }
type Profile struct {
	name     string
	injectIn map[string]bool
}

const defaultStr = "default"

// DefaultProfile specific default profile that allows anything to be injected into anything
var DefaultProfile = Profile{
	name: defaultStr,
	injectIn: map[string]bool{
		defaultStr: true,
	},
}

// NewAppProfile returns a new AppProfile that will drive
// injection and dependency throu your application
func NewAppProfile() *AppProfile {
	return &AppProfile{
		profiles: make(map[string]*Profile),
		locked:   false,
	}
}

// StrictHTTPAppProfile return a strict http profile based on :
//
// - handler : manage http input and can call service
//
// - service : manage business logic and can call repositories
//
// - repository : manage data access
//
// - driver : manage resource driver
//
func StrictHTTPAppProfile() *AppProfile {
	app := NewAppProfile()
	app.AddProfileDef("handler")
	app.AddProfileDef("service", "handler")
	app.AddProfileDef("repository", "service")
	app.AddProfileDef("driver", "repository")
	app.locked = true
	return app
}

// HTTPAppProfile return a less strict http profile based on :
//
// - handler : manage http input and can call services
//
// - service : manage business logic and can call repositories and other services
//
// - repository : manage data access
//
func HTTPAppProfile() *AppProfile {
	app := NewAppProfile()
	app.AddProfileDef("handler")
	app.AddProfileDef("service", "handler", "service")
	app.AddProfileDef("repository", "service")
	app.locked = true
	return app
}

// // NewProfile create a new profile
// // - name : the name of the profile
// // - injectIn : where it can be injected
// func NewProfile(name string, injectIn ...string) *Profile {
// 	injects := make(map[string]bool)
// 	for _, i := range injectIn {
// 		injects[i] = true
// 	}
// 	return &Profile{
// 		name:     name,
// 		injectIn: injects,
// 	}
// }

// AddProfileDef add a new profile definition
//
// - name name to use in injection
//
// - injectIn where it can be injected
//
func (ap *AppProfile) AddProfileDef(name string, injectIn ...string) error {
	injects := make(map[string]bool)
	for _, i := range injectIn {
		injects[i] = true
	}
	return ap.AddProfile(&Profile{
		name:     name,
		injectIn: injects,
	})

}

// AddProfile add a profile
func (ap *AppProfile) AddProfile(p *Profile) error {
	if ap.locked {
		return newError(fmt.Errorf("AppProfile locked, can't add new profile")).SetErrType(ErrTypeProfile)
	}
	if ap.profiles == nil {
		ap.profiles = map[string]*Profile{}
	}
	if ap.profiles[p.name] != nil {
		return newError(fmt.Errorf("%s is already declared", p.name)).SetErrType(ErrTypeProfile)
	}
	ap.profiles[p.name] = p
	return nil
}

func (ap *AppProfile) lock() error {
	if !ap.locked {
		if len(ap.profiles) == 0 {
			// inserting DefaultProfile
			ap.AddProfile(&DefaultProfile)
		} else {
			// default profile is prohibited if there is profiles definition
			l := len(ap.profiles)
			for key := range ap.profiles {
				if key == defaultStr && l > 1 {
					return newError(fmt.Errorf("default profile can't be used in a multi-profile app")).SetErrType(ErrTypeProfile)
				}
			}
		}
		ap.locked = true
	}
	return nil
}

// IsLocked AppProfile is locked after the first lifecycle phase or with some strict definition
func (ap *AppProfile) isLocked() bool {
	return ap.locked
}

func (p *Profile) canBeInjectedIn(other string) bool {
	return p.injectIn[other]
}

func (ap *AppProfile) validate(label string) bool {
	return ap.profiles[label] != nil
}

func (ap *AppProfile) isDefault() bool {
	return ap.profiles[defaultStr] != nil
}

func (ap *AppProfile) validateTag(label, itag string) (string, error) {
	if ap.isDefault() {
		return itag, nil
	}
	elts := strings.Split(itag, ":")
	if len(elts) != 2 {
		return "", newError(fmt.Errorf("wrong number of argument when declaring tag %s", itag)).SetErrType(ErrTypeProfile)
	}
	p, err := ap.getProfile(elts[0])
	if err != nil {
		return "", err
	}

	_, err = ap.getProfile(label)
	if err != nil {
		return "", err
	}
	if p.canBeInjectedIn(label) {
		return elts[1], nil
	}
	return "", newError(fmt.Errorf("%s can't be injected in %s", elts[0], label)).SetErrType(ErrTypeProfile)
}

func (ap *AppProfile) getProfile(label string) (*Profile, error) {
	p := ap.profiles[label]
	if p == nil {
		return nil, newError(fmt.Errorf("profile %s does not exist", label)).SetErrType(ErrTypeProfile)
	}
	return p, nil
}
