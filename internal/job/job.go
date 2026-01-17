package job

import (
	"fmt"
	"time"
)

// IdleWait waits for the remaining idleSeconds so Prometheus can notice that something is happening.
// If the job has already run longer than idleSeconds, it will not wait.
func IdleWait(jobStart time.Time, idleSeconds int) {
	if idleSeconds <= 0 {
		return
	}

	// Calculate remaining time to reach idleSeconds
	elapsed := time.Since(jobStart)
	remaining := time.Duration(idleSeconds)*time.Second - elapsed

	if remaining > 0 {
		fmt.Printf("Idle flag active, waiting for additional %v\n", remaining)
		time.Sleep(remaining)
	}
}
