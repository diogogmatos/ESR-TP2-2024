package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"ser.uminho/tp2/common"
	controlProtocol "ser.uminho/tp2/control-protocol"
	"ser.uminho/tp2/helpers"
	rtpServer "ser.uminho/tp2/rtp-streaming/server"
	tcp "ser.uminho/tp2/tcp-server"
)

var openConns map[net.Addr]net.Conn = make(map[net.Addr]net.Conn, 0)

type NodeData struct {
	address net.Addr
}

var Neighbours map[string]NodeData = make(map[string]NodeData)
var chronChannel = make(chan int)

var StreamingTable common.StreamingTable = common.NewStreamingTable()
var OngoingStreams map[string]string = make(map[string]string) // stream -> port
var MessagesAwaitingResponse *helpers.SafeMap[uint32, controlProtocol.Message] = helpers.NewMap[uint32, controlProtocol.Message]()

var RoutingTable = common.NewRoutingTable()

var RETRIES int = 3

func packetHandler(g *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {
	switch msg.Header.MsgType {

	case controlProtocol.MsgIWantStream:
		HandleIWantStream(g, conn, msg)
	case controlProtocol.MsgHello:
		HandleHello(g, conn, msg)
	case controlProtocol.MsgAcceptedConn:
		HandleAcceptedConn(g, conn, msg)
	case controlProtocol.MsgTimeHop:
		HandleTimeHop(g, conn, msg)
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

var TCPMan = tcp.NewGajo(handleConn)

func ListenControl() {
	helpers.OkLogger.Println("Listening on 0.0.0.0:8888")
	TCPMan.Listen("0.0.0.0:8888")
}

func HandleTimeHop(TCPMan *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {
	msg.Header.Flags.IsResponse = true
	packet, _ := msg.Encode()
	helpers.WarnLogger.Println(TCPMan.Send(conn.RemoteAddr(), packet))
}

// DONE
func HandleHello(TCPMan *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {

	var message controlProtocol.Message

	message.Header = controlProtocol.Header{
		NodeType:  controlProtocol.Server,
		MsgType:   controlProtocol.MsgHello, // Message type set to NewConn
		Id:        rand.Uint32(),
		TimeStamp: uint32(time.Now().Unix()),
	}
	encoded_message, _ := message.Encode()

	TCPMan.Send(conn.RemoteAddr(), []byte(encoded_message))

	fmt.Printf("new neighbour: %s\n", conn.RemoteAddr())
	addr := conn.RemoteAddr().(*net.TCPAddr)
	addr.Port = 8888
	Neighbours[conn.RemoteAddr().String()] = NodeData{address: addr}
}

func HandleAcceptedConn(TCPMan *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {
	streamId := string(msg.Body)
	streamId = strings.Trim(streamId, " \n\t")
	content := strings.Split(streamId, "]")[0][1:]
	helpers.OkLogger.Println(content)
	RoutingTable.AddEntry(common.RoutingEntry{From: nil, To: conn.RemoteAddr(), ConnId: streamId, InUse: true})
	go Stream(TCPMan, content, streamId)
}

func Stream(TCPMan *tcp.Gajo, content string, connId string) {
	helpers.WarnLogger.Println("startStream(", content, ",", connId, ")")
	_, ok := OngoingStreams[content]
	if !ok {
		StreamingTable.AddEntry(common.StreamingEntry{Conn: connId, Stream: content})
		channel := make(chan []byte)
		OngoingStreams[content] = content
		helpers.OkLogger.Println("started new stream for", content)
		go rtpServer.DupStream(content, channel, &StreamingTable, RoutingTable)
		rtpServer.PacketizeVideo(content, channel)
		// Stream acabou
		delete(OngoingStreams, content)
		StreamingTable.FilterInPlace(func(se common.StreamingEntry) bool { return se.Stream != content })
		RoutingTable.FilterInPlace(func(entry common.RoutingEntry) bool {
			if content == common.GetConnContent(entry.ConnId) {
				var message controlProtocol.Message

				message.Header = controlProtocol.Header{
					NodeType: controlProtocol.Server,
					MsgType:  controlProtocol.MsgEndOfStream,
					Flags:    controlProtocol.Flags{IsResponse: false, BackTrack: false, Forward: true},
				}

				message.SetRandId()
				message.SetTimeNow()
				message.SetBody([]byte(content))
				encoded_message, _ := message.Encode()
				TCPMan.Send(entry.To, encoded_message)

				return false
			}
			return true
		})
	} else {
		helpers.OkLogger.Println("stream for ", content, "already existed ")
	}
}

// DONE
func HandleIWantStream(TCPMan *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {
	if msg.Header.NodeType == controlProtocol.Client {
		content := string(msg.Body)
		_, err := os.Stat(content)
		if err != nil {
			var message controlProtocol.Message
			message.Header = controlProtocol.Header{
				NodeType:  controlProtocol.Server,
				MsgType:   controlProtocol.MsgError, // Message type set to NewConn
				Id:        msg.Header.Id,
				Flags:     controlProtocol.Flags{IsResponse: true},
				TimeStamp: msg.Header.TimeStamp,
			}
			message.SetTimeNow()
			message.SetBody([]byte("Content unavailable"))
			packet, _ := message.Encode()
			TCPMan.SendOpen(conn.RemoteAddr(), packet)
			return
		} else {
			var message controlProtocol.Message
			message.Header = controlProtocol.Header{
				NodeType:  controlProtocol.Server,
				MsgType:   controlProtocol.MsgOk, // Message type set to NewConn
				Id:        msg.Header.Id,
				Flags:     controlProtocol.Flags{IsResponse: true},
				TimeStamp: msg.Header.TimeStamp,
			}
			packet, _ := message.Encode()
			TCPMan.SendOpen(conn.RemoteAddr(), packet)

		}
		connId := strings.Trim(content, " \n\t")

		Id := "[" + connId + "]"
		n := 1

		for _, node := range Neighbours {
			Idn := Id
			Idn += fmt.Sprint("-", n)
			n++
			clientIP := conn.RemoteAddr()

			// convert net.IP to netip.Adress
			convertedClientIP, _ := netip.AddrFromSlice(clientIP.(*net.TCPAddr).IP)
			body := &controlProtocol.NewConnBody{ConnId: Idn, ClientIp: convertedClientIP, TotalHops: 0, SplitAt: 0, TotalTime: 0}
			msgBody, _ := json.Marshal(body)

			var message controlProtocol.Message
			message.Header = controlProtocol.Header{
				NodeType:  controlProtocol.Server,
				MsgType:   controlProtocol.MsgConnFlood, // Message type set to NewConn
				Id:        msg.Header.Id,
				Flags:     controlProtocol.Flags{IsResponse: true, Forward: true},
				TimeStamp: msg.Header.TimeStamp,
			}

			message.SetBody(msgBody)
			packet, _ := message.Encode()
			TCPMan.SendOpen(node.address, packet)
		}
	}
}

// DONE
func Announce() {
	for _, node := range Neighbours {
		helpers.OkLogger.Println("announcing to: ", node.address.String())
		go func() {
			for i := 0; i < RETRIES; i++ {
				waitTime := time.Duration(1<<i) * time.Second
				if i > 0 {
					time.Sleep(waitTime)
				}

				var message controlProtocol.Message

				message.Header = controlProtocol.Header{
					NodeType:  controlProtocol.Server,
					MsgType:   controlProtocol.MsgHello, // Message type set to NewConn
					Id:        rand.Uint32(),
					TimeStamp: uint32(time.Now().Unix()),
				}
				encoded_message, _ := message.Encode()

				var err error
				_, _, _, err = TCPMan.SendOpen(node.address, encoded_message)
				if err != nil {
					helpers.ErrorLogger.Println("Error announcing to", node.address, ";", err)
				}
			}
			helpers.ErrorLogger.Println("wasnt able to connect to:", node.address.String())
		}()
	}
	helpers.OkLogger.Println("Announcing step over")
}

var commands = map[string]func(any, []string) helpers.StatusMessage{
	"peers": func(_ any, a []string) helpers.StatusMessage {
		msg := helpers.NewStatusMessage()
		for k := range Neighbours {
			msg.AddMessage(nil, k)
		}
		return msg
	}, "conns": func(_ any, a []string) helpers.StatusMessage {
		msg := helpers.NewStatusMessage()
		for _, v := range openConns {

			m := v.LocalAddr().String() + "->" + v.RemoteAddr().String()

			msg.AddMessage(nil, m)
		}
		return msg
	},
	"streams": func(_ any, a []string) helpers.StatusMessage {
		msg := helpers.NewStatusMessage()
		for _, v := range StreamingTable {
			m := v.Stream + "->" + v.Conn
			msg.AddMessage(nil, m)
		}
		return msg
	}, "table": func(_ any, a []string) helpers.StatusMessage {
		msg := helpers.NewStatusMessage()
		RoutingTable.Show()
		return msg
	},
}

func chronFun() {
	for {
		for content, _ := range OngoingStreams {

			toSend := make(map[net.Addr]string)
			max := 0

			for _, re := range RoutingTable.ToTable() {
				if re.To != nil && common.GetConnContent(re.ConnId) == content {
					toSend[re.To] = re.ConnId
					if l := strings.Split(re.ConnId, "-"); len(l) > 1 {
						n := l[len(l)-1]
						m, err := strconv.Atoi(n)
						if m > max {
							max = m
						}
						if err != nil {
							continue
						}
					}
				}
			}

			for addr, conn := range toSend {
				body := &controlProtocol.NewConnBody{ConnId: conn, TotalHops: 0, SplitAt: 0, TotalTime: 0}
				msgBody, _ := json.Marshal(body)

				var message controlProtocol.Message
				message.Header = controlProtocol.Header{
					NodeType: controlProtocol.Server,
					MsgType:  controlProtocol.MsgConnFlood, // Message type set to NewConn
					Flags:    controlProtocol.Flags{Forward: true},
				}
				message.SetRandId()
				message.SetTimeNow()
				message.SetBody(msgBody)
				packet, _ := message.Encode()
				TCPMan.SendOpen(addr, packet)
			}

			Id := "[" + content + "]"
			n := max
			for _, node := range Neighbours {
				if _, ok := toSend[node.address]; ok {
					Idn := Id
					Idn += fmt.Sprint("-", n)
					n++

					body := &controlProtocol.NewConnBody{ConnId: Idn, TotalHops: 0, SplitAt: 0, TotalTime: 0}
					msgBody, _ := json.Marshal(body)

					var message controlProtocol.Message
					message.Header = controlProtocol.Header{
						NodeType: controlProtocol.Server,
						MsgType:  controlProtocol.MsgConnFlood, // Message type set to NewConn
						Flags:    controlProtocol.Flags{Forward: true},
					}
					message.SetRandId()
					message.SetTimeNow()
					message.SetBody(msgBody)
					packet, _ := message.Encode()
					TCPMan.SendOpen(node.address, packet)

				}

			}

		}
		time.Sleep(5 * time.Second)
	}
	return
}

func main() {
	godotenv.Load()
	retries := os.Getenv("RETRIES")
	if retries != "" {
		n, err := strconv.Atoi(retries)
		if err != nil {
			RETRIES = n
		}
	}
	for _, v := range os.Args[1:] {
		ip := net.ParseIP(v)
		port := 8888
		address := &net.TCPAddr{
			IP:   ip,
			Port: port,
		}
		Neighbours[v] = NodeData{address: address}
	}
	Announce()
	go ListenControl()
	go chronFun()
	reader := bufio.NewReader(os.Stdin)
	helpers.TUI(reader, nil, commands)

}
