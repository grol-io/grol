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

// Creates a new error object with the given message and stack.
func (s *State) Error(msg string) object.Error {
	if log.LogDebug() {
		log.LogVf("Error %q called", msg)
	}
	return object.Error{Value: msg, Stack: s.Stack()}
}

func (s *State) Errorf(format string, args ...interface{}) object.Error {
	return s.Error(fmt.Sprintf(format, args...))
}
