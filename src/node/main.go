package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net"

	// "net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/olekukonko/tablewriter"
	"github.com/pion/rtp"
	"ser.uminho/tp2/common"
	controlProtocol "ser.uminho/tp2/control-protocol"
	"ser.uminho/tp2/helpers"
	tcp "ser.uminho/tp2/tcp-server"
)

// TYPES

const (
	UNKNOWN = 0b00000000
	SRC     = 0b00000001
	DEST    = 0b00000010
	NODE    = controlProtocol.Node
	POP     = controlProtocol.POP
	SERVER  = controlProtocol.Server
	CLIENT  = controlProtocol.Client
)

type PeerData struct {
	Address net.Addr
	Type    byte
}

// GLOBALS

var KnownPeers map[string]PeerData = make(map[string]PeerData)
var MessagesAwaitingResponse *helpers.SafeMap[uint32, chan controlProtocol.Message]
var RoutingTable common.RoutingTable

var TYPE = NODE
var RETRIES = 3
var PANIC_BALANCE = 2

var chronChannel = make(chan int)

// Tratar Streams por UDP
func ListenStreams() {
	udpAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:8888")
	if err != nil {
		fmt.Println("Error resolving address:", err)
		return
	}

	// Create a UDP connection
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println("Error listening on UDP:", err)
		return
	}
	buffer := make([]byte, 80000)

	for {
		// Read from the connection
		n, _, _ := conn.ReadFromUDP(buffer)
		packet, connID := common.UnwrapUDPPacket(buffer[:n])

		// fmt.Println("received ", connID, "(", n, "bytes) from", src)
		rtpPacket := &rtp.Packet{}
		if err := rtpPacket.Unmarshal(packet); err != nil {
			helpers.ErrorLogger.Printf("Failed to unmarshal RTP packet: %v\n", err)
			continue
		}

		// Display the received frame
		// if frame.Empty() {
		// 	helpers.ErrorLogger.Println("corrupt frame")
		// }

		for _, entry := range RoutingTable.ToTable() {
			if entry.InUse && entry.To != nil && common.GetConnContent(entry.ConnId) == common.GetConnContent(connID) {
				sendConn, err := net.Dial("udp", strings.Split(entry.To.String(), ":")[0]+":8888")

				if err != nil {
					helpers.ErrorLogger.Println("error connecting:", err)
				}
				_, err = sendConn.Write(common.WrapUDPPacket(packet, connID))

				// fmt.Println("sent ", entry.ConnId, "to", entry.To)
				// n, err := sendConn.Write(common.WrapUDPPacket(packet, connID))
				if err != nil {
					helpers.ErrorLogger.Println("error sending", len(packet), "bytes")
					helpers.ErrorLogger.Println(err)
				}
				sendConn.Close()
			}
		}
	}
}

// Protocolo de Controlo

func handleConn(TCPMan *tcp.Gajo, conn net.Conn) {
	defer conn.Close()
	defer TCPMan.Remove(conn.RemoteAddr())
	err := tcp.PacketHandlerDispatcher(TCPMan, conn, packetHandler)
	handleDeath(TCPMan, conn.RemoteAddr(), err)
}

func handleDeath(TCPMan *tcp.Gajo, addr net.Addr, err error) {
	data, ok := KnownPeers[addr.String()]
	if !ok {
		return
	}
	delete(KnownPeers, addr.String())
	helpers.WarnLogger.Println(addr, ":{", data, "} has left due to", err)
	if data.Type == CLIENT {
		helpers.WarnLogger.Println("bro was a client")
		connsToClose := make([]string, 1)
		RoutingTable.FilterInPlace(func(re common.RoutingEntry) bool {
			if helpers.CompareAddrs(re.To, addr) {
				helpers.WarnLogger.Println("comparing", re.To, "and", addr)
				connsToClose = append(connsToClose, re.ConnId)
				return false
			}
			return !helpers.CompareAddrs(re.To, addr)
		})
		helpers.WarnLogger.Println("TABLE AFTER FILTERING")
		RoutingTable.Show()
	} else if data.Type == SERVER {
		helpers.ErrorLogger.Println("Server saiu, n posso fazer muito ._.")
		contentsFromServer := make(map[string]any)
		RoutingTable.FilterInPlace(func(re common.RoutingEntry) bool {
			if helpers.CompareIps(data.Address, re.From) {
				contentsFromServer[common.GetConnContent(re.ConnId)] = 1
				return false
			}
			return true
		})
		var message controlProtocol.Message

		message.Header = controlProtocol.Header{
			NodeType: controlProtocol.Server,
			MsgType:  controlProtocol.MsgEndOfStream,
			Flags:    controlProtocol.Flags{IsResponse: false, BackTrack: false, Forward: true},
		}

		message.SetRandId()
		message.SetTimeNow()

		RoutingTable.FilterInPlace(func(re common.RoutingEntry) bool {
			if _, ok := contentsFromServer[common.GetConnContent(re.ConnId)]; ok {
				if re.To != nil {
					message.SetBody([]byte(common.GetConnContent(re.ConnId)))
					encoded_message, _ := message.Encode()
					TCPMan.Send(re.To, encoded_message)
				}
				return false
			} else {
				return true
			}
		})
	} else {
		helpers.WarnLogger.Println("Was a Node, table before handling")
		RoutingTable.Show()
		connsToCheckSources := make(map[string]int, 1)
		connsToCheckDests := make(map[string]int, 1)
		removed := RoutingTable.FilterInPlace(func(re common.RoutingEntry) bool {
			if helpers.CompareIps(re.From, addr) {
				if re.InUse {
					connsToCheckDests[re.ConnId] = 1
					connsToCheckDests[common.GetConnParent(re.ConnId)] = 1
				}
				return false
			}
			if helpers.CompareIps(re.To, addr) {
				if re.InUse {
					connsToCheckSources[common.GetConnParent(re.ConnId)] = 1
				}
				return false
			}
			return true
		})

		print("conns to check Dests\n")
		for k, _ := range connsToCheckDests {
			print(k, "\n")
		}

		print("conns to check Sources\n")
		for k, _ := range connsToCheckSources {
			print(k, "\n")
		}

		dests := make(map[net.Addr]int, 0)

		removed = append(removed, RoutingTable.FilterInPlace(func(re common.RoutingEntry) bool {
			for c, _ := range connsToCheckDests {
				if strings.HasPrefix(re.ConnId, c) {

					if re.InUse {
						dests[re.To] = 1
					}
					helpers.WarnLogger.Println("removed row:", re.String())
					return false
				}
			}
			return true
		})...)

		removed = append(removed, RoutingTable.PruneOrphans()...)
		helpers.OkLogger.Println("Filtered & Pruned")
		RoutingTable.Show()
		helpers.OkLogger.Println("Removed")
		common.ShowRoutingEntryArray(removed)

		for _, re := range removed {
			if re.To != nil && helpers.CompareIps(re.To, addr) {

			}
		}

		toRestore := make([]common.RoutingEntry, 0)
		for _, re := range removed {
			if re.InUse && re.To != nil {
				toRestore = append(toRestore, re)
			}
		}

		helpers.OkLogger.Println("Need to restore")
		common.ShowRoutingEntryArray(toRestore)

		contentsToRestore := make(map[string][]net.Addr)

		for _, re := range toRestore {
			m, ok := contentsToRestore[common.GetConnContent(re.ConnId)]
			if re.To != nil {
				if !ok {
					contentsToRestore[common.GetConnContent(re.ConnId)] = []net.Addr{re.To}
				} else {
					contentsToRestore[common.GetConnContent(re.ConnId)] = append(m, re.To)
				}
			}

		}

		for cont, _ := range contentsToRestore {
			for _, source := range RoutingTable.GetContentSources(cont) {
				helpers.OkLogger.Println("potential source:", source)
				minHops := 100000
				conn := ""

				entryToAccept := common.RoutingEntry{}

				RoutingTable.Map(func(re common.RoutingEntry) common.RoutingEntry {
					if helpers.CompareIps(re.From, source) {
						if re.HopsSinceSplit < minHops {
							conn = re.ConnId
							minHops = re.HopsSinceSplit
							entryToAccept = re
						}
					}
					return re
				})

				var message controlProtocol.Message
				message.Header = controlProtocol.Header{
					NodeType: controlProtocol.Client,
					MsgType:  controlProtocol.MsgAcceptedConn,
					Id:       rand.Uint32(),
					Flags:    controlProtocol.Flags{BackTrack: true},
				}
				message.SetBody([]byte(conn))
				message.SetTimeNow()
				encoded_message, _ := message.Encode()
				TCPMan.Send(source, encoded_message)
				fmt.Println("SENT BACKTRACK TO", source)

				entryToAccept.InUse = true
				RoutingTable.AddEntry(entryToAccept)

				RoutingTable.Map(func(re common.RoutingEntry) common.RoutingEntry {
					if common.GetConnContent(re.ConnId) == cont {
						for _, ip := range contentsToRestore[cont] {
							if helpers.CompareIps(re.To, ip) {
								re.InUse = true
							}
						}
					}
					return re
				})

				break
			}
		}
	}
}

func packetHandler(g *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {
	if ch, ok := MessagesAwaitingResponse.Load(msg.Header.Id); ok {
		ch <- *msg
		return
	}
	switch msg.Header.MsgType {
	case controlProtocol.MsgHello:
		handleHello(g, conn, msg)
	case controlProtocol.MsgConnFlood:
		handleConnFlood(g, conn, msg)
	case controlProtocol.MsgAcceptedConn:
		handleAcceptedConn(g, conn, msg)
	case controlProtocol.MsgCloseConn:
		handleCloseConn(g, conn, msg)
	case controlProtocol.MsgEndOfStream:
		handleEndOfStream(g, conn, msg)
	case controlProtocol.MsgTimeHop:
		handleTimeHop(g, conn, msg)
	case controlProtocol.MsgIWantStream:
		HandleIWantStream(g, conn, msg)
	}
}

func handleHello(_ *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {
	helpers.OkLogger.Printf("New neighbour: %s\n", conn.RemoteAddr())

	addr := conn.RemoteAddr().(*net.TCPAddr)
	addr.Port, _ = strconv.Atoi(strings.Split(conn.RemoteAddr().String(), ":")[1])

	if msg.Header.NodeType == controlProtocol.Client {
		KnownPeers[conn.RemoteAddr().String()] = PeerData{Address: addr, Type: CLIENT}
		return
	}
	if msg.Header.NodeType&controlProtocol.Node != 0 {
		var Type byte = NODE
		if msg.Header.NodeType&controlProtocol.POP != 0 {
			Type |= controlProtocol.POP
		}
		KnownPeers[conn.RemoteAddr().String()] = PeerData{Address: addr, Type: Type}
		return
	}
	if msg.Header.NodeType&controlProtocol.Server != 0 {
		KnownPeers[conn.RemoteAddr().String()] = PeerData{Address: addr, Type: SERVER}
		return
	}
}

func handleTimeHop(TCPMan *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {
	msg.Header.Flags.IsResponse = true
	packet, _ := msg.Encode()
	TCPMan.SendOpen(conn.RemoteAddr(), packet)
}

func handleConnFlood(TCPMan *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {

	body := &controlProtocol.NewConnBody{}
	json.Unmarshal(msg.Body, body)
	body.TotalHops++
	body.SplitAt++
	if !RoutingTable.PassesThrough(body.ConnId) {
		hopTime := TimeHop(TCPMan, conn.RemoteAddr())
		body.TotalTime += hopTime
		RoutingTable.AddEntry(common.RoutingEntry{
			From:           conn.RemoteAddr(),
			To:             nil,
			ConnId:         body.ConnId,
			InUse:          false,
			TotalHops:      body.TotalHops,
			LastHopTime:    hopTime,
			TimeFromSource: body.TotalTime + hopTime})
		if TYPE == POP {
			clientAddress := &net.TCPAddr{
				IP:   net.IP(body.ClientIp.AsSlice()),
				Port: 8888,
			}
			KnownPeers[clientAddress.String()] = PeerData{Address: clientAddress, Type: CLIENT}
			RoutingTable.AddEntry(common.RoutingEntry{
				From:           conn.RemoteAddr(),
				To:             nil,
				ConnId:         body.ConnId,
				InUse:          false,
				TotalHops:      body.TotalHops,
				HopsSinceSplit: body.SplitAt,
			})
			body.ConnId = RoutingTable.AddOutgoingForConn(body.ConnId, clientAddress)
			msgBody, _ := json.Marshal(*body)

			message := *msg
			message.Header = controlProtocol.Header{
				NodeType: controlProtocol.Node | controlProtocol.POP,
				MsgType:  controlProtocol.MsgConnFlood, // Message type set to NewConn
			}
			message.SetTimeNow()
			message.SetBody(msgBody)
			packet, _ := message.Encode()
			TCPMan.SendOpen(clientAddress, packet)
		} else {
			helpers.WarnLogger.Println("Flooding...")
			for _, node := range KnownPeers {
				if !helpers.CompareIps(node.Address, conn.RemoteAddr()) && node.Type != SERVER {
					helpers.WarnLogger.Println("node:", node.Address)
					body2 := *body
					body2.ConnId = RoutingTable.AddOutgoingForConn(body.ConnId, node.Address)
					body2.TotalTime += hopTime
					message := *msg
					message.Header = controlProtocol.Header{
						NodeType: controlProtocol.Node,
						MsgType:  controlProtocol.MsgConnFlood,
					}

					message.SetTimeNow()
					msgBody, _ := json.Marshal(body2)
					message.SetBody(msgBody)
					packet, _ := message.Encode()
					TCPMan.SendOpen(node.Address, packet)
				}
			}
		}
	} else {
		helpers.WarnLogger.Println("DID NOT FLOOD")
	}
}

func handleCloseConn(TCPMan *tcp.Gajo, _ net.Conn, msg *controlProtocol.Message) {
	connId := string(msg.Body)
	msg.Header.NodeType = controlProtocol.Node
	count := 0
	if msg.Header.Flags.BackTrack {
		RoutingTable.Map(func(re common.RoutingEntry) common.RoutingEntry {
			if common.GetConnParent(connId) == common.GetConnParent(re.ConnId) && re.To != nil {
				count++
			}
			if re.ConnId == connId {
				if re.To != nil {
					re.InUse = false
				}
			}

			return re
		})
		if count <= 1 {
			helpers.WarnLogger.Println("No siblings")
			RoutingTable.Map(func(re common.RoutingEntry) common.RoutingEntry {
				if common.GetConnParent(connId) == re.ConnId {
					if re.From != nil {
						msg.SetBody([]byte(re.ConnId))
						message, _ := msg.Encode()
						TCPMan.Send(re.From, message)
					}
				}
				return re
			})

		}
	} else if msg.Header.Flags.Forward {
		RoutingTable.Map(func(re common.RoutingEntry) common.RoutingEntry {
			if connId == common.GetConnParent(re.ConnId) && re.To != nil {
				msg.SetBody([]byte(re.ConnId))
				message, _ := msg.Encode()
				TCPMan.Send(re.From, message)
			}
			if re.ConnId == connId {
				if re.From != nil {
					re.InUse = false
				}
			}

			return re
		})
	}
}

func handleEndOfStream(TCPMan *tcp.Gajo, _ net.Conn, msg *controlProtocol.Message) {
	content := string(msg.Body)
	msg.Header.NodeType = controlProtocol.Node
	RoutingTable.FilterInPlace(func(re common.RoutingEntry) bool {
		if common.GetConnContent(re.ConnId) == content {
			if re.To != nil {
				message, _ := msg.Encode()
				TCPMan.Send(re.To, message)
			}
			return false
		} else {
			return true
		}
	})
}

func handleAcceptedConn(TCPMan *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {
	connId := string(msg.Body)
	connId = strings.Trim(connId, " \n\t")
	helpers.OkLogger.Println("connid:", connId)
	RoutingTable.Map(func(entry common.RoutingEntry) common.RoutingEntry {
		entryConn := strings.Trim(entry.ConnId, " \n\t")
		if entry.ConnId == connId && helpers.CompareAddrs(entry.To, conn.RemoteAddr()) {
			entry.InUse = true
		}

		if entry.From != nil {
			if entryConn == connId || strings.HasPrefix(connId, entryConn) {
				entry.InUse = true

				var message controlProtocol.Message

				var nodeType uint8 = controlProtocol.Node

				if TYPE == POP {
					nodeType |= controlProtocol.POP
				}

				message.Header = controlProtocol.Header{
					NodeType: nodeType,
					MsgType:  controlProtocol.MsgAcceptedConn,
					Id:       rand.Uint32(),
				}
				message.SetTimeNow()
				message.SetBody([]byte(entryConn))
				encoded_message, _ := message.Encode()
				TCPMan.SendOpen(entry.From, encoded_message)

				helpers.OkLogger.Println("source is", entry.From.String())
			}
		}

		return entry
	})
	RoutingTable.Show()
	println("\n\n")
}

func HandleIWantStream(TCPMan *tcp.Gajo, conn net.Conn, msg *controlProtocol.Message) {
	RoutingTable.Show()
	content := string(msg.Body)
	fmt.Println("content:", content)

	entriesForContent := RoutingTable.GetIncomingForContent(content)
	fmt.Println("entriesForContent:", entriesForContent)
	activeEntriesForContent := make([]common.RoutingEntry, 0)
	for _, v := range entriesForContent {
		if v.InUse {
			activeEntriesForContent = append(activeEntriesForContent, v)
		}
	}
	fmt.Println("ActiveEntriesForContent:", activeEntriesForContent)
}

var TCPMan = tcp.NewGajo(handleConn)

func ListenControl() {
	helpers.OkLogger.Println("Listening on 0.0.0.0:8888")
	TCPMan.Listen("0.0.0.0:8888")
}

func Announce() {
	for _, node := range KnownPeers {
		helpers.OkLogger.Println("announcing to: ", node.Address.String())
		go func() {
			for i := 0; i < RETRIES; i++ {
				waitTime := time.Duration(1<<i) * time.Second
				if i > 0 {
					time.Sleep(waitTime)
				}
				conn, err := net.Dial("tcp", node.Address.String())
				if err != nil {
					helpers.WarnLogger.Println("Error connecting to "+node.Address.String()+":\n", err)
				} else {

					var message controlProtocol.Message
					var nodeType uint8 = controlProtocol.Node

					if TYPE == POP {
						nodeType |= controlProtocol.POP
					}

					message.Header = controlProtocol.Header{
						NodeType: nodeType,
						MsgType:  controlProtocol.MsgHello, // Message type set to NewConn
						Id:       rand.Uint32(),
					}
					message.SetTimeNow()
					encoded_message, _ := message.Encode()

					var err error
					if TYPE == POP {
						_, _, _, err = TCPMan.SendOpen(node.Address, []byte(encoded_message))
					} else {
						_, _, _, err = TCPMan.SendOpen(node.Address, []byte(encoded_message))
					}
					if err != nil {
						helpers.ErrorLogger.Println("Error announcing to", conn.RemoteAddr().String(), ";", err)
					}

					return
				}
			}
			helpers.ErrorLogger.Println("wasnt able to connect to:", node.Address.String())
		}()
	}
	helpers.OkLogger.Println("Announcing step over")
}

func isPanic() bool {
	sources := make(map[net.Addr]byte)
	dests := make(map[net.Addr]byte)
	RoutingTable.Map(func(re common.RoutingEntry) common.RoutingEntry {
		if re.From != nil {
			sources[re.From] = KnownPeers[re.From.String()].Type
		}
		if re.To != nil {
			dests[re.To] = 1
		}
		return re
	})

	if len(sources) <= 1 {
		for _, t := range sources {
			if t == SERVER {
				// helpers.WarnLogger.Println("not in panic, only one source, but it's the server")
				return false
			}
		}
		return len(dests) > 0
	}
	return false
}

// função que corre periodicamente
func chronFun() {
	return
	if len(RoutingTable.Entries) > 0 && isPanic() {
		helpers.ErrorLogger.Println("OH SHIT OH FUCK")
		sources := make(map[net.Addr]int)
		dests := make(map[net.Addr]int)

		RoutingTable.Map(func(re common.RoutingEntry) common.RoutingEntry {
			if re.From != nil {
				sources[re.From] = 1
			}
			if re.To != nil {
				dests[re.To] = 1
			}
			return re
		})

		for k, _ := range sources {
			helpers.WarnLogger.Println(k, "is a source")
		}
	} else {
		// helpers.OkLogger.Println("we ok, dw.")
	}
}

func TimeHop(TCPMan *tcp.Gajo, address net.Addr) int {
	var message controlProtocol.Message
	var nodeType uint8 = controlProtocol.Node

	if TYPE == POP {
		nodeType |= controlProtocol.POP
	}

	message.Header = controlProtocol.Header{
		NodeType: nodeType,
		MsgType:  controlProtocol.MsgTimeHop,
		Id:       rand.Uint32(),
	}
	message.SetTimeNow()
	encoded_message, _ := message.Encode()

	TCPMan.SendOpen(address, encoded_message)
	response_channel := make(chan controlProtocol.Message)
	MessagesAwaitingResponse.Store(message.Header.Id, response_channel)
	resp := <-response_channel
	MessagesAwaitingResponse.Delete(message.Header.Id)
	rtt := uint32(time.Now().UnixMilli()) - resp.Header.TimeStamp
	helpers.WarnLogger.Println("hop to", address, "took", rtt, "miliseconds")

	RoutingTable.Map(func(re common.RoutingEntry) common.RoutingEntry {
		if helpers.CompareIps(re.From, address) {

			re.LastHopTime = int(rtt / 2)
		}

		return re
	})

	return int(rtt)
}

func ShowType(t byte) string {
	types := []string{}

	// Check for SRC and DEST (can be combined with others)
	if t&SRC != 0 {
		types = append(types, "SRC")
	}
	if t&DEST != 0 {
		types = append(types, "DEST")
	}

	// Check for specific types (mutually exclusive or combined with SRC/DEST)
	if t&NODE != 0 {
		types = append(types, "NODE")
	}
	if t&POP != 0 {
		types = append(types, "POP")
	}
	if t&SERVER != 0 {
		types = append(types, "SERVER")
	}
	if t&CLIENT != 0 {
		types = append(types, "CLIENT")
	}

	// If no type is matched, it's UNKNOWN
	if len(types) == 0 {
		types = append(types, "UNKNOWN")
	}

	// Join types with a separator to handle combinations like "SRC | NODE"
	return strings.Join(types, " | ")
}

var commands = map[string]func(any, []string) helpers.StatusMessage{
	"peers": func(_ any, a []string) helpers.StatusMessage {
		msg := helpers.NewStatusMessage()
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Type", "Address"})

		// Add rows for each peer
		for k, peer := range KnownPeers {
			row := []string{ShowType(peer.Type), strings.Split(k, ":")[0]}
			table.Append(row)
		}
		table.Render()
		print(KnownPeers)
		return msg
	},
	"conns": func(_ any, a []string) helpers.StatusMessage {
		msg := helpers.NewStatusMessage()
		for _, v := range TCPMan.Table.ToMap() {
			m := v.LocalAddr().String() + "->" + v.RemoteAddr().String()
			msg.AddMessage(nil, m)
		}
		return msg
	},
	"table": func(_ any, a []string) helpers.StatusMessage {
		msg := helpers.NewStatusMessage()
		RoutingTable.Show()
		return msg
	},
}

func main() {
	godotenv.Load()
	retries := os.Getenv("RETRIES")
	if retries != "" {
		n, err := strconv.Atoi(retries)
		if err != nil {
			RETRIES = n
		}
	} else {
		RETRIES = 3
	}
	helpers.OkLogger.Println(os.Args)
	x := 1
	if len(os.Args) > x && os.Args[x][0] == '-' {
		if strings.EqualFold(os.Args[x], "-pop") {
			TYPE = POP
			x++
		}
	}

	MessagesAwaitingResponse = helpers.NewMap[uint32, chan controlProtocol.Message]()

	switch TYPE {
	case NODE:
		helpers.OkLogger.Print("Node\n\n")
	case POP:
		helpers.OkLogger.Print("POP\n\n")
	}
	if x < len(os.Args) {
		for _, v := range os.Args[x:] {
			ip := net.ParseIP(v)
			if ip != nil {
				port := 8888
				address := &net.TCPAddr{
					IP:   ip,
					Port: port,
				}
				KnownPeers[address.String()] = PeerData{Address: address, Type: UNKNOWN}
			} else {
				helpers.WarnLogger.Println("Error parsing '", v, "' as ip")
			}
		}
	}
	Announce()
	go ListenControl()
	go ListenStreams()
	go helpers.ChronJob(1*time.Second, chronFun, chronChannel)
	reader := bufio.NewReader(os.Stdin)
	helpers.TUI(reader, nil, commands)
}
