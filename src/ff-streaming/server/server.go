package ffServer

import (
	"fmt"
	"log"
	"net"
	"os/exec"
)

func Stream(video string, conn net.Conn) {

	defer conn.Close()

	fmt.Println(
		"ffmpeg", "-i", video, "-f", "mpegts", "-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
		"udp://"+conn.RemoteAddr().String())

	// FFmpeg command to stream a video file as H.264 over UDP
	ffmpegCmd := exec.Command(
		"ffmpeg", "-i", video, "-f", "mpegts", "-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
		"udp://"+conn.RemoteAddr().String())

	if err := ffmpegCmd.Run(); err != nil {
		log.Fatalf("Failed to run ffmpeg command: %v", err)
	}
	log.Println("Streaming video started")
}
