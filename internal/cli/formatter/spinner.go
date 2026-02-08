package formatter

import (
	"fmt"
	"sync"
	"time"
)

// Braille dot spinner frames.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner displays an animated spinner with a message in the terminal.
type Spinner struct {
	mu      sync.Mutex
	message string
	stop    chan struct{}
	done    chan struct{}
}

// NewSpinner creates a new spinner with the given message.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation. Call Stop() to end it.
func (s *Spinner) Start() {
	go func() {
		defer close(s.done)
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.stop:
				// Clear the spinner line.
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				frame := spinnerFrames[i%len(spinnerFrames)]
				fmt.Printf("\r  %s %s", StylePurple.Render(frame), Dim(s.message))
				i++
			}
		}
	}()
}

// Stop ends the spinner animation and clears the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.stop:
		// Already stopped.
		return
	default:
		close(s.stop)
	}
	<-s.done
}

// StartSpinner is a convenience function that creates, starts, and returns
// a spinner. Call the returned function to stop it.
func StartSpinner(message string) func() {
	s := NewSpinner(message)
	s.Start()
	return s.Stop
}
