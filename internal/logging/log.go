package logging

import "log"

// VerbosityKind represents logging verbosity
type VerbosityKind int

const (
	SilentVerbosity = 0
	ErrVerbosity    = 1
	InfVerbosity    = 2
	DbgVerbosity    = 3
	TraceVerbosity  = 4
)

// Verbosity is the current logging verbosity
var Verbosity VerbosityKind = InfVerbosity

// SetVerbosity sets the global verbosity from debug/trace switches.
func SetVerbosity(debug, trace bool) {
	switch {
	case trace:
		Verbosity = TraceVerbosity
	case debug:
		Verbosity = DbgVerbosity
	default:
		Verbosity = InfVerbosity
	}
}

// Lerr logs an error
func Lerr(format string, v ...interface{}) {
	if Verbosity >= ErrVerbosity {
		log.Printf("[ERROR] "+format, v...)
	}
}

// Linf logs an info message
func Linf(format string, v ...interface{}) {
	if Verbosity >= InfVerbosity {
		log.Printf("[INFO] "+format, v...)
	}
}

// Ldbg logs a debug message
func Ldbg(format string, v ...interface{}) {
	if Verbosity >= DbgVerbosity {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Ltrace logs a trace message
func Ltrace(format string, v ...interface{}) {
	if Verbosity >= TraceVerbosity {
		log.Printf("[TRACE] "+format, v...)
	}
}

// Lfatalf logs a fatal error and terminates the process.
func Lfatalf(format string, v ...interface{}) {
	log.Fatalf("[FATAL] "+format, v...)
}
