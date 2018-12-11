// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

import (
	"fmt"
)

type state int

const (
	stDeclaration state = iota
	stConfiguration
	stInjection
	stInitialization
	stRun
	stClose
)

func (s state) String() string {
	switch s {
	case 0:
		return fmt.Sprint("Declaration phase")
	case 1:
		return fmt.Sprint("Configuration phase")
	case 2:
		return fmt.Sprint("Injection phase")
	case 3:
		return fmt.Sprint("Initialization phase")
	case 4:
		return fmt.Sprint("Run phase")
	case 5:
		return fmt.Sprint("Close phase")
	}
	return fmt.Sprint("Unknown phase ")
}

type lifecycle struct {
	currentState state
	done         map[state]bool
}

func newLifecycle() *lifecycle {
	return &lifecycle{
		currentState: stDeclaration,
		done:         make(map[state]bool),
	}
}

func (l *lifecycle) current(st state) bool {
	return l.currentState == st
}

func (l *lifecycle) state() string {
	return l.currentState.String()
}

func (l *lifecycle) String() string {
	return l.state()
}
