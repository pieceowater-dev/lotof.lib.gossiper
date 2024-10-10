package panics

import (
	"github.com/rs/zerolog/log"
	"runtime/debug"
)

// DontPanic is a utility function that handles and logs panics.
// It recovers from a panic, if one occurs, and logs the panic message.
// This function is useful for ensuring that panics do not crash the application.
func DontPanic() {
	if r := recover(); r != nil {
		log.Printf("Recovered from panic: %v\n%s", r, debug.Stack())
	}
}
