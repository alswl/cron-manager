package job

import (
	"fmt"
	"time"
)

const (
	// IdleForSeconds is the number of seconds to idle for Prometheus to notice
	IdleForSeconds = 60
)

// IdleWait waits for the rest of the idleForSeconds so Prometheus can notice that something is happening
func IdleWait(jobStart time.Time) {
	// Idling to let Prometheus to notice we are running
	diff := IdleForSeconds - (time.Now().Unix() - jobStart.Unix())
	if diff > 0 {
		fmt.Printf("Idle flag active so I am going to wait for for additional %d seconds", diff)
		time.Sleep(time.Second * time.Duration(diff))
	}
}
