package controlProtocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand/v2"
	"net/netip"
	"strings"
	"time"
)

const (
	MsgError = iota
	MsgOk
	MsgIWantStream
	MsgEndOfStream
	MsgHello
	MsgConnFlood
	MsgAcceptedConn
	MsgCloseConn
	MsgTimeHop
	MsgTimeConn
)

// Node types
const (
	Server = 0b0001
	Client = 0b0010
	Node   = 0b0100
	POP    = 0b1000
)

type Flags struct {
	IsResponse bool
	BackTrack  bool
	Forward    bool
}

func (f *Flags) Show() string {
	return fmt.Sprintf(
		"Response:  %v \n"+
			"BackTrack: %v \n"+
			"Forward:   %v \n", f.IsResponse, f.BackTrack, f.Forward,
	)
}

type Header struct {
	NodeType    uint8
	MsgType     uint8
	Flags       Flags
	TimeStamp   uint32
	Id          uint32
	Hash        uint16
	PayloadSize uint16
}

type Message struct {
	Header Header
	Body   []byte
}

var Hasher = fnv.New32a()

func EncodeHeader(header *Header) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write NodeType
	if err := binary.Write(buf, binary.LittleEndian, header.NodeType); err != nil {
		return nil, err
	}

	// Write MsgType
	if err := binary.Write(buf, binary.LittleEndian, header.MsgType); err != nil {
		return nil, err
	}

	// Write Flags
	if err := binary.Write(buf, binary.LittleEndian, header.Flags); err != nil {
		return nil, err
	}

	// Write TimeStamp
	if err := binary.Write(buf, binary.LittleEndian, header.TimeStamp); err != nil {
		return nil, err
	}

	// Write Id
	if err := binary.Write(buf, binary.LittleEndian, header.Id); err != nil {
		return nil, err
	}

	// Write Hash
	if err := binary.Write(buf, binary.LittleEndian, header.Hash); err != nil {
		return nil, err
	}

	// Write PayloadSize
	if err := binary.Write(buf, binary.LittleEndian, header.PayloadSize); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DecodeHeader(r io.Reader) (*Header, error) {
	var header Header

	// Read fixed-size fields into the header struct
	if err := binary.Read(r, binary.LittleEndian, &header.NodeType); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &header.MsgType); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &header.Flags); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &header.TimeStamp); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &header.Id); err != nil {
		return nil, err
	}
	// Read IP address (4 bytes)
	if err := binary.Read(r, binary.LittleEndian, &header.Hash); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &header.PayloadSize); err != nil {
		return nil, err
	}

	return &header, nil
}

// EncodeMessage encodes the entire Message struct into a byte slice.
func (msg *Message) Encode() ([]byte, error) {
	// Encode the header
	headerBytes, err := EncodeHeader(&msg.Header)
	if err != nil {
		return nil, err
	}

	// Combine the header and body
	buf := new(bytes.Buffer)
	buf.Write(headerBytes)
	buf.Write(msg.Body)

	return buf.Bytes(), nil
}

func DecodeMessage(r io.Reader) (*Message, error) {

	// Decode the header
	header, err := DecodeHeader(r)
	if err != nil {
		return nil, err
	}

	// Read the payload (Body)
	body := make([]byte, header.PayloadSize)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}

	return &Message{Header: *header, Body: body}, nil
}

func ShowNodeType(nodeType uint8) string {
	if nodeType == Server {
		return "Server"
	}
	if nodeType == Client {
		return "Client"
	}
	if nodeType&Node != 0 {
		if nodeType&POP != 0 {
			return "POP"
		}
		return "Node"
	}
	return "Unknown"
}

func ShowMsgType(msgType uint8) string {
	switch msgType {
	case MsgError:
		return "Error"
	case MsgOk:
		return "Ok"
	case MsgIWantStream:
		return "I Want Stream"
	case MsgHello:
		return "Hello"
	case MsgConnFlood:
		return "Conn Flood"
	case MsgAcceptedConn:
		return "Accepted Conn"
	case MsgCloseConn:
		return "Close Conn"
	case MsgTimeHop:
		return "Time Hop"
	case MsgEndOfStream:
		return "End of Stream"
	}

	return "Unkown"
}

func (m *Message) Show() string {

	header := m.Header
	body := m.Body

	indentedFlags := ""
	for _, line := range strings.Split(header.Flags.Show(), "\n") {
		indentedFlags += "\t" + line + "\n"
	} // indentedFlags = header.Flags.Show()
	// Create a human-readable string representation
	return fmt.Sprintf(
		"{\n"+
			"  NodeType:     %s\n"+
			"  Message Type: %s\n"+
			"  Flags {\n"+
			"%s"+
			"  }\n"+
			"  Timestamp:    %d\n"+
			"  ID:           %d\n"+
			"  Hash:         0x%04x\n"+
			"  Payload Size: %d\n"+
			"  Body:         %s\n"+
			"}\n",
		ShowNodeType(header.NodeType),
		ShowMsgType(header.MsgType),
		indentedFlags,
		header.TimeStamp,
		header.Id,
		header.Hash,
		header.PayloadSize,
		string(body),
	)
}

func (m *Message) SetPayloadSize() {
	m.Header.PayloadSize = uint16(len(m.Body))
}

func (m *Message) SetBody(body []byte) {
	m.Header.PayloadSize = uint16(len(body))
	m.Body = body
}

func (m *Message) SetTimeNow() {
	m.Header.TimeStamp = uint32(time.Now().UnixMilli())
}
func (m *Message) SetRandId() {
	m.Header.Id = rand.Uint32()
}

type NewConnBody struct {
	ConnId    string     `json:conn_id`
	ClientIp  netip.Addr `json:client_ip`
	TotalHops int        `json:t_hops`
	SplitAt   int        `json:s_hops`
	TotalTime int        `json:total_time`
}
