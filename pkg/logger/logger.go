package logger

import (
	"fmt"
	"log"
)

type VerboseLogger struct{}

func (l *VerboseLogger) PhaseStart(phase string, totalItems int) {
	log.Printf("[%s] Starting phase with %d items", phase, totalItems)
}

func (l *VerboseLogger) ItemProcessed(phase string, item string, action string) {
	log.Printf("[%s] %s: %s", phase, action, item)
}

func (l *VerboseLogger) PhaseComplete(phase string, processedItems int) {
	log.Printf("[%s] Phase complete. Processed %d items", phase, processedItems)
}

type NullLogger struct{}

func (l *NullLogger) PhaseStart(phase string, totalItems int) {}

func (l *NullLogger) ItemProcessed(phase string, item string, action string) {}

func (l *NullLogger) PhaseComplete(phase string, processedItems int) {}

type QuietLogger struct{}

func (l *QuietLogger) PhaseStart(phase string, totalItems int) {}

func (l *QuietLogger) ItemProcessed(phase string, item string, action string) {
	if action != "skip" {
		fmt.Printf("%s: %s\n", action, item)
	}
}

func (l *QuietLogger) PhaseComplete(phase string, processedItems int) {}
