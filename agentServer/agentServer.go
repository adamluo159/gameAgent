package agentServer

import (
	"log"
	"net"
	"strconv"

	"github.com/adamluo159/gameAgent/protocol"
)

// Client holds info about connection
type Client struct {
	conn *net.Conn
	host string
}

// TCP server
type server struct {
	clients map[string]*Client
	address string // Address to open connection: localhost:9999
}

var gserver *server

// Start network server
func (s *server) Listen() {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		log.Fatal("Error starting TCP server.")
	}
	defer listener.Close()
	//go s.CheckTimeout()

	for {
		conn, _ := listener.Accept()
		client := &Client{
			conn: &conn,
		}
		go client.OnMessage()
	}
}

// Creates new tcp server instance
func New(address string) {
	log.Println("Creating server with address", address)
	gserver = &server{
		address: address,
		clients: make(map[string]*Client),
	}
	gserver.Listen()
}

func StartZone(host string, zid int) int {
	req := protocol.GetReqIndex()
	log.Println(" recv web cmd startzone", host, " zid:", zid, "req:", req)
	c := gserver.clients[host]
	if c == nil {
		log.Println(" cannt find host:", host)
		return protocol.NotifyDoFail
	}
	p := protocol.S2cNotifyDo{
		Name: "zone" + strconv.Itoa(zid),
		Req:  req,
	}
	err := protocol.SendJson(c.conn, protocol.CmdStartZone, p)
	if err != nil {
		log.Println(host + "  startzone: " + err.Error())
		return protocol.NotifyDoFail
	}
	s := protocol.C2sNotifyDone{}
	protocol.WaitCallBack(p.Req, &s)
	log.Println("start zone aaaaa:", s)
	return s.Do
}

func StopZone(host string, zid int) int {
	req := protocol.GetReqIndex()
	log.Println(" recv web cmd stopzone", host, " zid:", zid, "req:", req)
	c := gserver.clients[host]
	if c == nil {
		return protocol.NotifyDoFail
	}
	p := protocol.S2cNotifyDo{
		Name: "zone" + strconv.Itoa(zid),
		Req:  req,
	}
	err := protocol.SendJson(c.conn, protocol.CmdStopZone, p)
	if err != nil {
		log.Println(host + "  Stopzone: " + err.Error())
		return protocol.NotifyDoFail
	}
	s := protocol.C2sNotifyDone{}
	protocol.WaitCallBack(p.Req, &s)
	return s.Do
}

func Update(host string) {
	//log.Println(" recv web cmd update", host)
	//c := gserver.clients[host]
	//if c == nil {
	//	log.Println("cannt find client hostname:", host, gserver.clients)
	//	return
	//}
	//err := c.SendBytesCmd("update")
	//if err != nil {
	//	log.Println(host + "  update: " + err.Error())
	//}
}
