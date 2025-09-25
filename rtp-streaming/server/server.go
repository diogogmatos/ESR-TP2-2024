package rtpServer

import (
	"fmt"
	"log"
	"time"

	"github.com/pion/rtp"
	"gocv.io/x/gocv"
)

const FPS int = 30

type RtpStreamingError struct {
	Type byte
}

func (e RtpStreamingError) Error() string {
	switch e.Type {
	case UnableToOpen:
		return "UnableToOpen"
	}
	return "Uknown RTP Streaming Error"
}

const (
	UnableToOpen = iota
)

func PacketizeVideo(videoFile string, channel chan []byte) error {
	// Open the video file
	video, err := gocv.VideoCaptureFile(videoFile)
	if err != nil {
		return RtpStreamingError{UnableToOpen}
	}
	defer video.Close()

	// Create a matrix for frames
	img := gocv.NewMat()
	defer img.Close()

	// Initialize RTP packet
	packet := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 0,
			Timestamp:      0,
			SSRC:           1234,
		},
	}

	for {
		if ok := video.Read(&img); !ok || img.Empty() {
			fmt.Println("End of video file or empty frame")
			return nil
		}

		buf, err := gocv.IMEncodeWithParams(gocv.JPEGFileExt, img, []int{gocv.IMWriteJpegQuality, 10})
		if err != nil {
			log.Printf("Error encoding frame: %v", err)
			continue
		}

		packet.Payload = buf.GetBytes()
		packet.SequenceNumber++
		packet.Timestamp += uint32(90000 / FPS) // assuming 30 FPS

		data, err := packet.Marshal()
		if err != nil {
			log.Fatalf("Failed to marshal RTP packet: %v", err)
		}

		// fmt.Printf("generated %+v\n", packet)

		channel <- data

		time.Sleep(time.Second / time.Duration(FPS))
	}
}
