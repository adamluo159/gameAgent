package agentServer

import (
	"log"
	"net"
	"strconv"

	"time"

	"github.com/adamluo159/admin-react/server/comInterface"
	"github.com/adamluo159/gameAgent/protocol"
	"github.com/adamluo159/gameAgent/utils"
)

// Client holds info about connection
type Client struct {
	conn           *net.Conn
	host           string
	curSerivceDo   map[int]bool
	zoneServiceMap map[string]bool
}

// TCP server
type Aserver struct {
	clients             map[string]*Client
	address             string // Address to open connection: localhost:9999
	mhMgr               comInterface.MachineMgr
	zoneDBServiceMap    map[string][]string
	zonelogDBserviceMap map[string][]string
}

var gserver *Aserver

// Start network server
func (s *Aserver) Listen() {
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
func New(address string) *Aserver {
	log.Println("Creating server with address", address)
	gserver = &Aserver{
		address:             address,
		clients:             make(map[string]*Client),
		zoneDBServiceMap:    make(map[string][]string),
		zonelogDBserviceMap: make(map[string][]string),
	}
	return gserver
}

func (s *Aserver) Init(m comInterface.MachineMgr) {
	s.mhMgr = m
	allMs := m.GetAllMachines()
	for _, v := range allMs {
		s.UpdataDBFromMachine(v.Hostname, v.Applications)
	}
}

func (s *Aserver) UpdataDBFromMachine(host string, apps []string) {
	gserver.zoneDBServiceMap[host] = make([]string, 0)
	gserver.zonelogDBserviceMap[host] = make([]string, 0)

	for index := 0; index < len(apps); index++ {
		s := apps[index]
		t := utils.AgentServiceType(s, protocol.TserviceReg)
		if t == protocol.TzoneDB {
			gserver.zoneDBServiceMap[host] = append(gserver.zoneDBServiceMap[host], s)
		} else if t == protocol.TzonelogDB {
			gserver.zonelogDBserviceMap[host] = append(gserver.zonelogDBserviceMap[host], s)
		}
	}
}

func (s *Aserver) ClientDisconnect(host string) {
	delete(s.clients, host)
}

func (s *Aserver) StartZone(host string, zid int) int {
	req := protocol.GetReqIndex()
	log.Println(" recv web cmd startzone", host, " zid:", zid, "req:", req)
	c := (*s).clients[host]
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
	r := protocol.C2sNotifyDone{}
	protocol.WaitCallBack(p.Req, &r, 30)
	return r.Do
}

func (s *Aserver) StopZone(host string, zid int) int {
	req := protocol.GetReqIndex()
	log.Println(" recv web cmd stopzone", host, " zid:", zid, "req:", req)
	c := (*s).clients[host]
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
	r := protocol.C2sNotifyDone{}
	protocol.WaitCallBack(p.Req, &r, 30)
	return r.Do
}

func (s *Aserver) UpdateZone(host string) int {
	log.Println(" recv web cmd update", host)
	req := protocol.GetReqIndex()
	c := gserver.clients[host]
	if c == nil {
		log.Println("cannt find client hostname:", host, gserver.clients)
		return protocol.NotifyDoFail
	}
	p := protocol.S2cNotifyDo{
		Name: "updateZone",
		Req:  req,
	}
	err := protocol.SendJson(c.conn, protocol.CmdUpdateHost, p)
	if err != nil {
		log.Println(host + "  updatezone: " + err.Error())
		return protocol.NotifyDoFail
	}
	r := protocol.C2sNotifyDone{}
	protocol.WaitCallBack(p.Req, &r, 30)
	return r.Do
}

func (s *Aserver) StartAllZone() bool {
	for _, v := range s.clients {
		go v.HostNotifyDo(protocol.CmdStartHostZone, protocol.Tzone)
	}
	suc := false
	for index := 0; index < 30; index++ {
		for _, v := range s.clients {
			if v.curSerivceDo[protocol.Tzone] == false {
				break
			}
			suc = true
		}
		time.Sleep(time.Second * 10)
	}
	return suc
}

func (s *Aserver) OnlineZones() map[string]*map[string]bool {
	onlinezs := make(map[string]*map[string]bool)
	for _, v := range s.clients {
		onlinezs[v.host] = &v.zoneServiceMap
	}
	return onlinezs
}

func (s *Aserver) CheckOnlineMachine(mName string) bool {
	if _, ok := (*s).clients[mName]; ok {
		return true
	}
	return false
}
