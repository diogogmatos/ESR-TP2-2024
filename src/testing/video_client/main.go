package main

import (
	"log"
	"net"

	ffplayClient "ser.uminho/tp2/ff-streaming/client"
)

func main() {
	addr := "localhost:8888" // Address to listen for the video stream
	_, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Fatalf("Failed to set up UDP connection: %v", err)
	}
	ffplayClient.WatchStream("localhost:8888")
}
