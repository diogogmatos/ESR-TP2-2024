package helpers

import "time"

func ChronJob(cadence time.Duration, procedure func(), ch chan int) {
	ticker := time.NewTicker(cadence)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C: // Execute the procedure on the cadence
			procedure()
		case <-ch: // Execute the procedure when something is written to the channel
			procedure()
		}
	}
}
