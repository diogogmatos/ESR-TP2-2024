// client.go
package rtpClient

import (
	"fmt"
	"log"
	"net"

	"github.com/pion/rtp"
	"gocv.io/x/gocv"
	"ser.uminho/tp2/common"
)

func ListenStream() {
	// Prepare to receive RTP packets
	var seq uint16 = 0
	conn, err := net.ListenPacket("udp", "0.0.0.0:8888")
	if err != nil {
		log.Fatalf("Failed to open UDP connection: %v", err)
	}
	defer conn.Close()

	// Initialize image matrix and window for display
	img := gocv.NewMat()
	defer img.Close()
	window := gocv.NewWindow("RTP Video Stream")
	defer window.Close()

	// Buffer for incoming data
	buf := make([]byte, 80000)

	for {
		n, src, err := conn.ReadFrom(buf)
		buf, connId := common.UnwrapUDPPacket(buf)

		if err != nil {
			log.Printf("Error receiving RTP packet: %v", err)
			continue
		}

		// Parse the RTP packet
		packet := &rtp.Packet{}
		if err := packet.Unmarshal(buf[:n]); err != nil {
			log.Printf("Failed to unmarshal RTP packet: %v", err)
			continue
		}
		seq = packet.SequenceNumber
		if packet.SequenceNumber > seq {
		// Decode the JPEG payload to an image
		frame, err := gocv.IMDecode(packet.Payload, gocv.IMReadColor)
		fmt.Printf("received %+v, %d bytes :%s from %s\n", packet, n, connId, src.String())
		if err != nil {
			log.Printf("Failed to decode image: %v", err)
			continue
		}

		// Display the received frame
		if frame.Cols() != 0 {
			window.IMShow(frame)
			if window.WaitKey(1) >= 0 {
				break
			}
		} else {
			print("empty frame")
		}
	}
	}
}
