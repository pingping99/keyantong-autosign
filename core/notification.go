package core

import "log"

type Notifier interface {
	Notify(message string)
}

type LogNotifier struct{}

func (notifier *LogNotifier) Notify(message string) {
	log.Printf("[CRITICAL_FAILURE] %s", message)
}

var GlobalNotifier Notifier = &LogNotifier{}

func Notify(message string) {
	if GlobalNotifier != nil {
		GlobalNotifier.Notify(message)
	}
}
