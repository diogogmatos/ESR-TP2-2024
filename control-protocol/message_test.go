package controlProtocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"reflect"
	"testing"
)

func TestEncodeMessage(t *testing.T) {
	header := Header{
		NodeType:    1,
		MsgType:     2,
		Flags:       Flags{true, true, false},
		TimeStamp:   1627568390,
		Id:          12345,
		Hash:        789,
		PayloadSize: 4,
	}

	body := []byte("test")
	msg := Message{
		Header: header,
		Body:   body,
	}

	// Encode the message
	encoded, err := msg.Encode()
	if err != nil {
		t.Fatalf("Failed to encode message: %v", err)
	}

	// Validate the header
	expectedHeader := new(bytes.Buffer)
	binary.Write(expectedHeader, binary.LittleEndian, header.NodeType)
	binary.Write(expectedHeader, binary.LittleEndian, header.MsgType)
	binary.Write(expectedHeader, binary.LittleEndian, header.Flags)
	binary.Write(expectedHeader, binary.LittleEndian, header.TimeStamp)
	binary.Write(expectedHeader, binary.LittleEndian, header.Id)
	binary.Write(expectedHeader, binary.LittleEndian, header.Hash)
	binary.Write(expectedHeader, binary.LittleEndian, header.PayloadSize)

	expectedMessage := append(expectedHeader.Bytes(), body...)

	// Compare the entire encoded message
	if !bytes.Equal(encoded, expectedMessage) {
		t.Errorf("Encoded message does not match expected output.\nGot:      %x\nExpected: %x", encoded, expectedMessage)
	}
}

func TestEncodeDecodeMessage(t *testing.T) {

	// Create a header
	header := Header{
		NodeType:    1,
		MsgType:     2,
		Flags:       Flags{true, true, false}, // Example: using a uint16 to store the flags
		TimeStamp:   1627568390,
		Id:          12345,
		Hash:        789,
		PayloadSize: 4,
	}

	// Create a body
	body := []byte("test")

	// Create a message
	msg := Message{
		Header: header,
		Body:   body,
	}

	// Encode the message
	encoded, err := msg.Encode()
	if err != nil {
		t.Fatalf("Failed to encode message: %v", err)
	}

	// Decode the message
	decodedMessage, err := DecodeMessage(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Failed to decode message: %v", err)
	}

	// Check if the decoded message matches the original
	if !reflect.DeepEqual(*decodedMessage, msg) {
		t.Errorf("Decoded message does not match the original.\nOriginal: %+v\nDecoded: %+v", msg, *decodedMessage)
	}
}

type Example struct {
	Field1 uint8
	Field2 uint16
	Field3 string
}

func TestNewConn(t *testing.T) {
	ex := NewConnBody{
		ConnId:    "A",
		ClientIp:  netip.AddrFrom4([4]byte{128, 0, 0, 1}),
		TotalHops: 3,
	}

	// Manually create the expected byte array

	// Use StructToBytes to convert the struct to bytes
	bytes, _ := json.Marshal(ex)

	ex2 := &NewConnBody{}
	json.Unmarshal(bytes, ex2)
	// Compare the actual output with the expected byte array
	if !reflect.DeepEqual(ex, *ex2) {
		t.Errorf("Decoded message does not match the original.\nOriginal: %+v\nDecoded: %+v", ex, ex2)
	}
}
