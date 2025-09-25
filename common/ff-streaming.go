package common

import (
	"fmt"
	"net"
)

func ServerDuper(content string, conn net.Conn, streamingTable *StreamingTable, routingTable *RoutingTable) {
	print("Duping")
	// Create a UDP connection to listen for incoming packets
	defer conn.Close()

	// Buffer for reading UDP packets
	buffer := make([]byte, 80000)

	for {
		// Read UDP packet
		n, err := conn.Read(buffer)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			continue
		}
		packet := buffer[:n]
		fmt.Println("read", n, "bytes")
		// Find all connections that need this stream
		for _, entry := range *streamingTable {
			if entry.Stream == content {
				// For each connection in the streaming table, get the associated IPs in the routing table
				for _, routeEntry := range routingTable.ToTable() {
					if routeEntry.ConnId == entry.Conn {
						// Send packet to each IP address in the routing table for this stream

						sendConn, err := net.DialUDP("udp", nil, routeEntry.To.(*net.UDPAddr))

						print("SEND CONN", sendConn.RemoteAddr().String())
						if err != nil {
							fmt.Println("Error dialing UDP connection:", err)
							continue
						}

						_, err = sendConn.Write(packet)
						if err != nil {
							fmt.Println("Error sending packet:", err)
						}
						sendConn.Close()
					}
				}
			}
		}
	}
}
