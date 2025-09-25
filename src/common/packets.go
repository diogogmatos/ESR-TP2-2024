package common

func WrapUDPPacket(packet []byte, streamId string) []byte {
	p := append([]byte{byte(len(streamId))}, []byte(streamId)...)
	p = append(p, packet...)
	return p
}

func UnwrapUDPPacket(wrappedPacket []byte) (packet []byte, streamId string) {
	b := wrappedPacket[0]
	length := int(b)
	streamId = string(wrappedPacket[1 : length+1])
	packet = wrappedPacket[length+1:]
	return packet, streamId
}
