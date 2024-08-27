package eval

import (
	"fmt"

	"fortio.org/log"
	"grol.io/grol/object"
)

// Returns a stack array of the current stack.
func (s *State) Stack() []string {
	stack := make([]string, 0, s.depth) // will be -1 because no name at toplevel but... it's fine.
	e := s.env
	for {
		if e == nil {
			break
		}
		n := e.Name()
		if n != "" {
			stack = append(stack, e.Name())
		}
		e = e.Parent()
	}
	log.Debugf("Stack() len %d, depth %d returning %v", len(stack), s.depth, stack)
	return stack
}

// Creates a new error object with the given message and stack.
func (s *State) Error(msg string) object.Error {
	return object.Error{Value: msg, Stack: s.Stack()}
}

func (s *State) Errorf(format string, args ...interface{}) object.Error {
	return s.Error(fmt.Sprintf(format, args...))
}
