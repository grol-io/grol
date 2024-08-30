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
	limited := LimitStack(stack, 10)
	log.Debugf("Stack() len %d, depth %d returning %v", len(stack), s.depth, limited)
	return limited
}

// Will use top and bottom N/2 elements of the stack to create a string.
func LimitStack(stack []string, limit int) []string {
	if len(stack) <= limit {
		return stack
	}
	limited := make([]string, 0, limit+1)
	limit /= 2
	limited = append(limited, stack[:limit]...)
	limited = append(limited, fmt.Sprintf("... %d more ...", len(stack)-limit*2))
	limited = append(limited, stack[len(stack)-limit:]...)
	return limited
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
// If the error is nil, it returns object.NULL instead (no error).
func (s *State) Error(err error) object.Object {
	if err == nil {
		return object.NULL
	}
	return s.NewError(err.Error())
}

/*
// IsNilError checks if an error is nil, including if it's a non-nil interface wrapping a nil value.
func IsNilError(err error) bool {
	if err == nil {
		return true
	}
	// Get the reflected value of the error
	val := reflect.ValueOf(err)

	// Check if the error is a pointer or interface and if it is nil
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface:
		return val.IsNil()
	default:
		return false
	}
}
*/
