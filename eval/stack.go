package eval

import (
	"fmt"

	"fortio.org/log"
	"grol.io/grol/object"
)

// Returns a stack array of the current stack.
func (s *State) Stack() []string {
	if s.depth <= 1 {
		return nil
	}
	stack := make([]string, 0, s.depth-1)
	for e := s.env; e != nil; e = e.StackParent() {
		n := e.Name()
		if n != "" {
			stack = append(stack, e.Name())
		}
	}
	log.Debugf("Stack() len %d, depth %d returning %v", len(stack), s.depth, stack)
	return stack
}

// NewError creates a new error object from a plain string.
// NewError will attach the stack trace to the Error object.
func (s *State) NewError(msg string) object.Error {
	if log.LogVerbose() {
		log.LogVf("Error %q called", msg)
	}
	return object.Error{Value: msg, Stack: s.Stack()}
}

// Errorf formats and create an object.Error using given format and args.
func (s *State) Errorf(format string, args ...interface{}) object.Error {
	return s.NewError(fmt.Sprintf(format, args...))
}

// Error converts from a go error to an object.Error.
func (s *State) Error(err error) object.Error {
	return s.NewError(err.Error())
}
