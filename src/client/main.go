package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"os"

	"github.com/joho/godotenv"
	"ser.uminho/tp2/common"
	controlProtocol "ser.uminho/tp2/control-protocol"
	"ser.uminho/tp2/helpers"
	rtpClient "ser.uminho/tp2/rtp-streaming/client"
	tcp "ser.uminho/tp2/tcp-server"
)

const (
	STARTING = iota
	AWAITING_STREAM
	RECEIVING_STREAM
)

var Stream string
var SeqNumber int
var Status = STARTING

type NodeData struct {
	ip       net.IP
	port     string
	isServer bool `default:false`
}

var Neighbours map[string]NodeData = make(map[string]NodeData)

var MessagesAwaitingResponse *helpers.SafeMap[uint32, chan controlProtocol.Message]

var RoutingTable common.RoutingTable
var TCPMan = tcp.NewGajo(handleConn)

// DONE
func handleNewConn(TCPMan *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {

	body := &controlProtocol.NewConnBody{}
	json.Unmarshal(msg.Body, body)
	body.TotalHops++

	if Status == STARTING {
		var message controlProtocol.Message
		message.Header = controlProtocol.Header{
			NodeType: controlProtocol.Client,
			MsgType:  controlProtocol.MsgAcceptedConn,
			Id:       rand.Uint32(),
		}
		message.SetBody([]byte(body.ConnId))
		message.SetTimeNow()
		encoded_message, _ := message.Encode()
		TCPMan.SendOpen(conn.RemoteAddr(), encoded_message)
		Status = AWAITING_STREAM
	}
}

func packetHandler(g *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {
	ch, ok := MessagesAwaitingResponse.Load(msg.Header.Id)
	if ok {
		ch <- *msg
		return
	}

	switch msg.Header.MsgType {
	case controlProtocol.MsgConnFlood:
		handleNewConn(g, conn, msg)
	}
}

func handleConn(TCPMan *tcp.Gajo, conn net.Conn) {
	defer conn.Close()
	defer TCPMan.Remove(conn.RemoteAddr())
	err := tcp.PacketHandlerDispatcher(TCPMan, conn, packetHandler)
	if err == io.EOF {
		helpers.WarnLogger.Println("TCP connection closed", conn.RemoteAddr().String())
	} else {
		helpers.ErrorLogger.Println("Some error happened when reading from TCP connection: ", err)
	}
}

func ListenControl() {
	helpers.OkLogger.Println("Listening on 0.0.0.0:8888")
	TCPMan.Listen("0.0.0.0:8888")
}

// DONE
func askForStream(serverIp net.IP, stream string) {
	if stream[len(stream)-1] == '\n' {
		stream = stream[:len(stream)-1]
	}

	var message controlProtocol.Message
	message.Header = controlProtocol.Header{
		NodeType: controlProtocol.Client,
		MsgType:  controlProtocol.MsgIWantStream, // Message type set to NewConn
	}
	message.SetRandId()
	message.SetBody([]byte(stream))
	message.SetTimeNow()
	encoded_message, _ := message.Encode()
	TCPMan.SendOpen(&net.TCPAddr{IP: serverIp, Port: 8888}, []byte(encoded_message))

	response_channel := make(chan controlProtocol.Message)
	MessagesAwaitingResponse.Store(message.Header.Id, response_channel)
	resp := <-response_channel
	MessagesAwaitingResponse.Delete(message.Header.Id)
	if resp.Header.MsgType == controlProtocol.MsgError {
		log.Fatal(string(resp.Body))
	} else {
		helpers.ErrorLogger.Println("HUH???")
	}
}

var commands = map[string]func(any, []string) helpers.StatusMessage{
	"peers": func(_ any, a []string) helpers.StatusMessage {
		msg := helpers.NewStatusMessage()
		for k, v := range Neighbours {
			var m string
			if v.isServer {
				m = "server:" + k
			} else {
				m = k
			}
			msg.AddMessage(nil, m)
		}
		return msg
	},
}

func main() {
	godotenv.Load()

	serverIp := net.ParseIP(os.Args[1])
	if serverIp == nil {
		os.Exit(1)
	}
	streamId := os.Args[2]

	MessagesAwaitingResponse = helpers.NewMap[uint32, chan controlProtocol.Message]()

	go ListenControl()
	askForStream(serverIp, streamId)
	go rtpClient.ListenStream()
	reader := bufio.NewReader(os.Stdin)
	helpers.TUI(reader, nil, commands)

}
