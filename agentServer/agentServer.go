package agentServer

import (
	"log"
	"net"
)

// Client holds info about connection
type Client struct {
	conn *net.Conn
	host string
}

// TCP server
type server struct {
	clients                  map[string]*Client
	address                  string // Address to open connection: localhost:9999
	onClientConnectionClosed func(c *Client, err error)
	onNewMessage             func(c *Client, message string)
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
	//msgMap = make(map[string]func(c *Client, a *AgentMsg))
	//msgMap["token"] = TokenCheck
	//msgMap["ping"] = Ping

	gserver.Listen()
}

func StartZone(host string, zid int) bool {
	//log.Println(" recv web cmd startzone", host, " zid:", zid)
	//c := gserver.clients[host]
	//if c == nil {
	//	return false
	//}
	//zone := "zone" + strconv.Itoa(zid)
	//err := c.SendBytes("start", zone)
	//if err != nil {
	//	log.Println(host + "  startzone: " + err.Error())
	//}
	return true
}

func StopZone(host string, zid int) bool {
	//log.Println(" recv web cmd stopzone", host, " zid:", zid)
	//c := gserver.clients[host]
	//if c == nil {
	//	return false
	//}
	//zone := "zone" + strconv.Itoa(zid)
	//err := c.SendBytes("stop", zone)
	//if err != nil {
	//	log.Println(host + "  stopzone: " + err.Error())
	//}
	return true
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
