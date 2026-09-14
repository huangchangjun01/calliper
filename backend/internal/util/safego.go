// Package util provides small, shared helpers used across the backend.
package util

import (
	"log"
	"runtime/debug"
)

// SafeGo runs fn in a new goroutine and recovers from any panic, logging the
// stack trace. This ensures a single background goroutine can never crash the
// whole process. Usage: util.SafeGo(func(){ myRoutine() })
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[panic recovered] %v\n%s", r, debug.Stack())
			}
		}()
		fn()
	}()
}
