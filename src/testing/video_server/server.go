package main

import (
	"net"

	ffServer "ser.uminho/tp2/ff-streaming/server"
)


func main(){
	conn, _ := net.Dial("udp", "localhost:8888")
	ffServer.Stream("video.mp4",conn)
}