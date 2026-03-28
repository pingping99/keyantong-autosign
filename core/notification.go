package core

import "log"

// Notifier defines an interface for sending alerts.
type Notifier interface {
	Notify(msg string)
}

// LogNotifier is a simple implementation that logs to standard output.
type LogNotifier struct{}

// Notify logs the message with a CRITICAL_FAILURE tag.
func (n *LogNotifier) Notify(msg string) {
	log.Printf("[CRITICAL_FAILURE] %s", msg)
}

// GlobalNotifier is the default notifier.
var GlobalNotifier Notifier = &LogNotifier{}

// Notify is a convenience function to use the global notifier.
func Notify(msg string) {
	if GlobalNotifier != nil {
		GlobalNotifier.Notify(msg)
	}
}
