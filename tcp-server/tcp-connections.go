package tcpConnections

import (
	"bufio"
	"log"
	"net"

	controlProtocol "ser.uminho/tp2/control-protocol"
	"ser.uminho/tp2/helpers"
)

type Gajo struct {
	Handler func(*Gajo, net.Conn)
	Table   helpers.SafeMap[net.Addr, net.Conn]
}

func NewGajo(handler func(*Gajo, net.Conn)) Gajo {
	return Gajo{
		handler,
		helpers.SafeMap[net.Addr, net.Conn]{},
	}
}

func (openConns *Gajo) OpenConn(address net.Addr) (net.Conn, error) {
	conn, err := net.Dial("tcp", address.String())
	if err != nil {
		return nil, err
	}
	openConns.Table.Store(conn.RemoteAddr(), conn)
	return conn, nil
}

func (openConns *Gajo) AddConn(conn net.Conn) {
	openConns.Table.Store(conn.RemoteAddr(), conn)
}

func (openConns *Gajo) Remove(address net.Addr) {

	openConns.Table.Delete(address)
}

type ConnTableError struct {
	Address net.Addr
}

func (err *ConnTableError) Error() string {
	return "no connection opened to" + err.Address.String()
}

func (openConns *Gajo) Send(address net.Addr, msg []byte) (int, error) {
	conn, ok := openConns.Table.Load(address)
	if !ok {
		return 0, &ConnTableError{Address: address}
	}
	return conn.Write(msg)
}

/*
(n sent, error, opened new conn)
*/
func (g *Gajo) SendOpen(address net.Addr, msg []byte) (int, net.Conn, bool, error) {
	conn, ok := g.Table.Load(address)
	var err error
	if !ok {
		conn, err = g.OpenConn(address)
	}

	if err != nil {
		return -1, nil, false, err
	}
	go g.Handler(g, conn)

	n, e := g.Send(conn.RemoteAddr(), msg)
	return n, conn, !ok, e

}

func (g *Gajo) Handle(conn net.Conn) {
	g.AddConn(conn)
	go g.Handler(g, conn)
}

type PacketHandler func(*Gajo, net.Conn, *controlProtocol.Message)

func PacketHandlerDispatcher(TCPMan *Gajo, conn net.Conn, packetHanlder PacketHandler) error {
	defer conn.Close()
	defer TCPMan.Remove(conn.RemoteAddr())

	// Create a reader for the connection
	reader := bufio.NewReader(conn)

	for {
		// Read data from the connection until a newline or error
		msg, err := controlProtocol.DecodeMessage(reader)
		if err != nil {
			return err
		}
		helpers.ReaderLogger.Printf("Received message: %s, from %s\n", msg.Show(), conn.RemoteAddr().String())

		packetHanlder(TCPMan, conn, msg)

	}
}

func (g *Gajo) Listen(address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	helpers.NormalLogger.Println("Listening on 0.0.0.0:8888")

	for {
		// Accept incoming connections
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}

		helpers.NormalLogger.Println("accepted connection from: ", conn.RemoteAddr())
		g.Handle(conn)
	}
}
