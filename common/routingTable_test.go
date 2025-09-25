package common

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
)

// Test the JSON serialization of RoutingTable and RoutingEntry
func TestRoutingTableSerialization(t *testing.T) {
	ip := net.ParseIP("100.200.200.100")
	port := 8888
	address := &net.TCPAddr{
		IP:   ip,
		Port: port,
	}
	// Create a sample RoutingTable
	rt := RoutingTable{
		Entries: []RoutingEntry{
			{
				From:           address, // Placeholder for net.Addr (you can use a real net.Addr here)
				To:             nil,     // Placeholder for net.Addr (you can use a real net.Addr here)
				ConnId:         "1234",
				InUse:          true,
				TotalHops:      5,
				HopsSinceSplit: 3,
				LastHopTime:    123456,
				TimeFromSource: 78910,
			},
		},
	}

	// Serialize the struct into JSON
	serialized, err := json.Marshal(rt)
	print(string(serialized))
	if err != nil {
		t.Fatalf("Error serializing struct: %v", err)
	}

	json.Unmarshal(serialized, &rt)
	fmt.Printf("%+v\n", rt)
	s := "a"
	fmt.Print(strings.Split(s, ":"))
}

func TestGetConnContent(t *testing.T) {
	content := GetConnContent("[video]-1-2-3-4")
	if content != "video" {
		t.Fatalf("conent: %s", content)
	}
}
func TestGetConnParent(t *testing.T) {
	if par := GetConnParent("[video]-1-2-3-4"); par != "[video]-1-2-3" {
		t.Fatalf("conent: %s", par)
	}
}
