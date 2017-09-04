package agentClient

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"encoding/json"

	"github.com/adamluo159/gameAgent/protocol"
	"github.com/adamluo159/gameAgent/utils"
)

type (

	//服务信息
	ServiceInfo struct {
		Sname          string //服务名
		Started        bool   //游戏区服是否已启动
		RegularlyCheck bool   //是否开启定时检查进程功能
		Operating      bool
	}

	Agent struct {
		conn      *net.Conn
		msgMap    map[uint32]func([]byte)
		srvs      map[string]*ServiceInfo //agent起的服务信息
		logdbIP   map[string]string
		logPhpArg map[string]string
	}

	Conf struct {
		ConAddress    string
		RemoteIP      string
		RemoteConfDir string
		LocalConfDir  string
		ProductDir    string
		CgPhp         string
		CmdSvnVer     string
		PhpTemplate   string
	}

	LogdbConf struct {
		DirName string
		IP      string
	}
)

var (
	conf Conf

	hostName string
	SvnVer   string
)

func UpdateSvn() {
	ver, err := utils.ExeShell("sh", conf.CmdSvnVer, "")
	if err != nil {
		log.Fatal("update svn ", err)
	}
	SvnVer = ver
}

func UpdateGameConf() {
	_, err := utils.ExeShellArgs3("expect", "./synGameConf_expt", conf.RemoteIP, conf.RemoteConfDir, conf.LocalConfDir)
	if err != nil {
		log.Fatal("Update gameConf:", err)
	}
}

func LoadConfig() {
	hostName, _ = os.Hostname()
	if hostName == "" {
		log.Println("cannt get machine hostname")
	}

	data, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Fatal(err)
	}

	datajson := []byte(data)
	err = json.Unmarshal(datajson, &conf)
	if err != nil {
		log.Fatal(err)
	}
	conf.RemoteConfDir += hostName

	UpdateSvn()
	UpdateGameConf()
	os.Mkdir(conf.LocalConfDir, os.ModePerm)
}

func LoadLogFile(agent *Agent) {
	hostDir := conf.LocalConfDir + hostName
	dir, err := ioutil.ReadDir(hostDir)
	if err != nil {
		log.Println("LoadLogFile, read dir err, ", err.Error())
	}
	for index := 0; index < len(dir); index++ {
		serviceName := dir[index].Name()

		file := hostDir + "/" + serviceName + "/logdbconf"
		l, err := ioutil.ReadFile(file)
		if err != nil {
			log.Println("LoadLogFile, read file err, ", file, err.Error())
		}
		db := LogdbConf{}
		jerr := json.Unmarshal(l, &db)
		if jerr != nil {
			log.Println("LoadLogFile uncode json", jerr.Error())
		}
		agent.logdbIP[db.DirName] = db.IP
		agent.logPhpArg[db.DirName] = fmt.Sprintf(conf.PhpTemplate, db.IP, db.DirName, "Crontab.dataProcess", db.IP)

		agent.InitSrv(serviceName)
		log.Println("LoadLogFile, logs:, ", db.DirName, agent.logPhpArg[db.DirName])
	}
	go agent.GameLogFileToDB()
}

func New() *Agent {
	LoadConfig()
	a := Agent{
		srvs:      make(map[string]*ServiceInfo),
		msgMap:    make(map[uint32]func([]byte)),
		logdbIP:   make(map[string]string),
		logPhpArg: make(map[string]string),
	}

	LoadLogFile(&a)

	a.msgMap[protocol.CmdToken] = a.S2cCheckRsp
	a.msgMap[protocol.CmdStartZone] = a.S2cStartZone
	a.msgMap[protocol.CmdStopZone] = a.S2cStopZone
	a.msgMap[protocol.CmdUpdateHost] = a.S2cUpdateZoneConfig
	a.msgMap[protocol.CmdStartHostZone] = a.S2cStartHostZones
	a.msgMap[protocol.CmdStopHostZone] = a.S2cStopHostZones
	a.msgMap[protocol.CmdNewZone] = a.S2cNewZone
	a.msgMap[protocol.CmdUpdateSvn] = a.S2cUpdateSvn

	a.RegularlyCheckProcess()

	return &a

}

//目前只有zone级服务初始化,后面添加登陆、充值等
func (agent *Agent) InitSrv(name string) {
	if _, ok := agent.srvs[name]; ok {
		return
	}

	run := CheckProcess(name)
	agent.srvs[name] = &ServiceInfo{
		Started:        run,
		RegularlyCheck: run,
		Sname:          name,
	}
}

func (agent *Agent) Connect() {
	for {
		conn, err := net.Dial("tcp", conf.ConAddress)
		if err != nil {
			log.Println("agent conn fail, addr:", conf.ConAddress)
			return
		}
		defer conn.Close()

		agent.conn = &conn

		agent.C2sCheckReq()

		// 消息缓冲
		msgbuf := bytes.NewBuffer(make([]byte, 0, 1024))
		// 数据缓冲
		databuf := make([]byte, 1024)
		// 消息长度
		length := 0

		for {
			// 读取数据
			n, err := conn.Read(databuf)
			if err == io.EOF {
				log.Printf("Client exit: %s\n", conn.RemoteAddr())
			}
			if err != nil {
				log.Printf("Read error: %s\n", err)
				return
			}
			// 数据添加到消息缓冲
			n, err = msgbuf.Write(databuf[:n])
			if err != nil {
				log.Printf("Buffer write error: %s\n", err)
				return
			}
			// 消息分割循环
			for {
				cmd, data := protocol.UnPacket(&length, msgbuf)
				if cmd <= 0 {
					break
				}
				mfunc := agent.msgMap[cmd]
				if mfunc == nil {
					log.Printf("cannt find msg handle server cmd: %d data: %s\n", cmd, string(data))
				} else {
					mfunc(data)
					log.Printf("server cmd: %d data: %s\n", cmd, string(data))
				}
			}
		}

		time.Sleep(5 * time.Second)
	}
}

//同步本机信息(机器名、机器上服务以及起停状态、svn代码版本号)
func (agent *Agent) C2sCheckReq() {
	p := protocol.C2sToken{
		Mservice: make(map[string]bool),
	}

	p.Host = hostName
	p.Token = utils.CreateMd5("cgyx2017")
	p.CodeVersion = SvnVer

	for k, v := range agent.srvs {
		p.Mservice[k] = v.Started
	}

	protocol.SendJson(agent.conn, protocol.CmdToken, &p)
}

func (agent *Agent) S2cCheckRsp(data []byte) {
	r := string(data)
	if r != "OK" {
		log.Fatal("register agentserver callback not ok")
	}
	//p := protocol.S2cToken{}
	//err := json.Unmarshal(data, &p)
	//if err != nil {
	//	log.Println("CheckRsp, uncode error:", string(data), err.Error())
	//	return
	//}
}

func (agent *Agent) StartZone(zone string) int {
	log.Println("recv start zone, zone:", zone)

	run := CheckProcess(zone)
	if run == false {
		utils.ExeShellArgs3("sh", conf.ProductDir+"/cgServer", "start", zone, "")
		for index := 0; index < 6; index++ {
			if run {
				break
			}
			run = CheckProcess(zone)
			time.Sleep(time.Second * 5)
		}
	}
	ret := protocol.NotifyDoFail
	agent.srvs[zone].Started = run
	if run == true {
		agent.srvs[zone].RegularlyCheck = true
		ret = protocol.NotifyDoSuc
	}
	agent.S2cZoneState(zone)
	return ret
}

func (agent *Agent) StopZone(zone string) int {
	log.Println("recv start zone, zone:", zone)
	run := CheckProcess(zone)
	if run == false {
		agent.srvs[zone].Started = false
		agent.srvs[zone].RegularlyCheck = false

		return protocol.NotifyDoSuc
	}
	utils.ExeShellArgs2("sh", conf.ProductDir+"/cgServer", "stop", zone)
	run = CheckProcess(zone)
	if run {
		return protocol.NotifyDoFail
	}

	agent.srvs[zone].Started = false
	agent.srvs[zone].RegularlyCheck = false
	agent.S2cZoneState(zone)
	return protocol.NotifyDoSuc
}

func (agent *Agent) S2cUpdateZoneConfig(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Stop Zone uncode json err, zone:", err.Error())
		return
	}
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoFail,
	}

	log.Println("update zoneConfig, Name:", p.Name, "req:", p.Req)
	UpdateGameConf()

	r.Do = protocol.NotifyDoSuc
	protocol.SendJson(agent.conn, protocol.CmdUpdateHost, r)
}

func (agent *Agent) S2cStartZone(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" StartZone uncode json err, zone:", err.Error())
		return
	}
	zone := p.Name
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoFail,
	}

	if agent.srvs[zone].Operating {
		r.Do = protocol.NotifyDoing
		protocol.SendJson(agent.conn, protocol.CmdStartZone, r)
		return
	}
	agent.srvs[zone].Operating = true
	r.Do = agent.StartZone(zone)
	agent.srvs[zone].Operating = false
	protocol.SendJson(agent.conn, protocol.CmdStartZone, r)
}

func (agent *Agent) S2cStopZone(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Stop Zone uncode json err, zone:", err.Error())
		return
	}
	zone := p.Name
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoFail,
	}
	log.Println("recv stop msg, Name:", zone, "req:", p.Req)
	if agent.srvs[zone].Operating {
		r.Do = protocol.NotifyDoing
		protocol.SendJson(agent.conn, protocol.CmdStopZone, r)
		return
	}
	agent.srvs[zone].Operating = true
	r.Do = agent.StopZone(zone)
	agent.srvs[zone].Operating = false
	protocol.SendJson(agent.conn, protocol.CmdStopZone, r)
}

func (agent *Agent) S2cStartHostZones(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Start hostZones uncode json err, zone:", err.Error())
		return
	}
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoSuc,
	}

	for k, _ := range agent.srvs {
		ret := agent.StartZone(k)
		if ret != protocol.NotifyDoSuc {
			r.Do = ret
		}
	}
	protocol.SendJson(agent.conn, protocol.CmdStartHostZone, r)
}

func (agent *Agent) S2cStopHostZones(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Stop hostZones uncode json err, zone:", err.Error())
		return
	}
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoSuc,
	}
	for k, _ := range agent.srvs {
		ret := agent.StopZone(k)
		if ret != protocol.NotifyDoSuc {
			r.Do = ret
		}

	}
	protocol.SendJson(agent.conn, protocol.CmdStopHostZone, r)
}

func (agent *Agent) S2cZoneState(zone string) {
	p := protocol.C2sZoneState{
		Zone: zone,
		Open: agent.srvs[zone].Started,
	}
	err := protocol.SendJson(agent.conn, protocol.CmdZoneState, p)
	if err != nil {
		log.Println("sysn zone state err, ", err.Error())
	}
}

func (agent *Agent) S2cNewZone(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Stop Zone uncode json err, zone:", err.Error())
		return
	}
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoSuc,
	}
	log.Println("update zoneConfig, Name:", p.Name, "req:", p.Req)

	UpdateGameConf()
	agent.InitSrv(p.Name)

	r.Do = protocol.NotifyDoSuc
	protocol.SendJson(agent.conn, protocol.CmdNewZone, r)
}

func (agent *Agent) S2cUpdateSvn(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Stop Zone uncode json err, zone:", err.Error())
		return
	}
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoSuc,
	}
	log.Println("update zoneConfig, Name:", p.Name, "req:", p.Req)

	cmdExe := conf.ProductDir + "/agent/svnUp"
	result, exeErr := utils.ExeShell("sh", cmdExe, "")
	if exeErr != nil {
		log.Println("Update cannt work!, reason:", exeErr.Error())
		r.Do = protocol.NotifyDoFail
	} else {
		cmdExe = conf.ProductDir + "/agent/svnInfo"
		result, exeErr = utils.ExeShell("sh", cmdExe, "")
		if exeErr != nil {
			log.Println("Update cannt work!, reason:", exeErr.Error())
			r.Do = protocol.NotifyDoFail
		} else {
			r.Do = protocol.NotifyDoSuc
			r.Result = result
			SvnVer = r.Result
		}
	}
	protocol.SendJson(agent.conn, protocol.CmdNewZone, r)
}
