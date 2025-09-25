package ffplayClient

import (
	"log"
	"os/exec"
)

func WatchStream(address string) {
	//defer conn.Close()

	// ffplay command to play the incoming video stream
	ffplayCmd := exec.Command("ffplay", "-f", "mpegts", "-i", "udp://"+address)

	if err := ffplayCmd.Start(); err != nil {
		log.Fatalf("Failed to start ffplay: %v", err)
	}
	log.Println("Video streaming started on client")
	ffplayCmd.Wait()
}
