package logger

// Logger defines the logging interface used by business logic.
// This abstraction allows different logging implementations to be used.
type Logger interface {
	Infof(msg string, args ...interface{})
	Warnf(msg string, args ...interface{})
	Errorf(msg string, args ...interface{})
	Debugf(msg string, args ...interface{})
}

// QuietLogger is a Logger that only writes important messages (errors, successful results).
// Other messages (info, debug) are suppressed for quiet operation outside sign windows.
type QuietLogger interface {
	QuietInfof(msg string, args ...interface{})
	Logger
}
