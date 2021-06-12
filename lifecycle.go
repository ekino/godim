// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

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
		return "declaration phase"
	case 1:
		return "configuration phase"
	case 2:
		return "injection phase"
	case 3:
		return "initialization phase"
	case 4:
		return "run phase"
	case 5:
		return "close phase"
	}
	return "Unknown phase "
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

func (l *lifecycle) String() string {
	return l.currentState.String()
}
