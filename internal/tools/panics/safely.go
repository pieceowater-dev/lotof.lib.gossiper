package panics

import "fmt"

// Safely runs a function with panic recovery and returns any errors.
// It executes the provided function and catches any panic that occurs.
// If a panic happens, it returns an error detailing the panic.
//
// Parameters:
//
//	fn - A function to be executed safely.
//
// Returns:
//
//	An error if a panic occurred; otherwise, nil.
func Safely(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occurred: %v", r)
		}
	}()
	fn()
	return
}
