package rtpServer

import (
	"log"
	"net"

	"ser.uminho/tp2/common"
)

func DupStream(content string, channel chan []byte, streamingTable *common.StreamingTable, routingTable *common.RoutingTable) {
	for packet := range channel {
		// For each connection in the streaming table, get the associated IPs in the routing table
		for _, routeEntry := range routingTable.ToTable() {
			if common.GetConnContent(routeEntry.ConnId) == content && routeEntry.InUse {
				// Send packet to each IP address in the routing table for this stream
				sendConn, err := net.Dial("udp", routeEntry.To.String())
				if err != nil {
					log.Println("Error dialing UDP connection:", err)
					continue
				}
				packet = common.WrapUDPPacket(packet, routeEntry.ConnId)
				_, err = sendConn.Write(packet)

				// helpers.NormalLogger.Println("sent ", routeEntry.ConnId, "(", n, "bytes) to", routeEntry.To)
				if err != nil {
					log.Println("Error sending packet:", err)
				}
				sendConn.Close()
			}
		}
	}
}
