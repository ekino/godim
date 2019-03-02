// Copyright 2018 ekino.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package godim

// ErrType base type for Error Type
type ErrType uint64

const (
	// ErrTypeInjection happens in injection phase failure
	ErrTypeInjection ErrType = 1 << 63
	// ErrTypeProfile happens in profile definition failure
	ErrTypeProfile ErrType = 1 << 62
	// ErrTypeRegistry happens in internal registry failure
	ErrTypeRegistry ErrType = 1 << 61
	// ErrTypeGodim happens in internal godim failure
	ErrTypeGodim ErrType = 1 << 60
	// ErrTypeEvent happens in internal event switch
	ErrTypeEvent ErrType = 1 << 59
	// ErrTypeAny for any other kind of errors
	ErrTypeAny ErrType = 1 << 1
)

// Error main godim error struct
type Error struct {
	Err  error
	Type ErrType
}

// Error from error interface.
func (err Error) Error() string {
	return err.Err.Error()
}

// SetErrType sets the error's type.
func (err *Error) SetErrType(er ErrType) *Error {
	err.Type = er
	return err
}

// IsErrType check kind of error.
func (err *Error) IsErrType(er ErrType) bool {
	return (err.Type & er) > 0
}

var _ error = &Error{}

func newError(err error) *Error {
	return &Error{Err: err}
}
