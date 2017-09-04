package agentClient

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"encoding/json"

	"github.com/adamluo159/gameAgent/protocol"
	"github.com/adamluo159/gameAgent/utils"
)

type (

	//服务信息
	ServiceInfo struct {
		Sname     string //服务名
		Started   bool   //游戏区服是否已启动
		Gof       bool   //是否开启定时检查进程功能
		Operating bool
		Tservice  int
	}

	Agent struct {
		conn          *net.Conn
		msgMap        map[uint32]func([]byte)
		agentServices map[string]*ServiceInfo //agent起的服务信息
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
)

type LogsInfo struct {
	logdbIP   map[string]string
	logPhpArg map[string]string
}

type LogdbConf struct {
	DirName string
	IP      string
}

var (
	conf Conf

	msgMap           map[uint32]func([]byte)
	hostName         string
	logConfs         LogsInfo
	codeVersion      string
	CheckProcessName = map[int]string{
		protocol.Tzone: "checkZoneProcess",
	}
)

//func RegCmd() {
//	logConfs.logdbIP = make(map[string]string)
//	logConfs.logPhpArg = make(map[string]string)
//
//	agent.agentServices = make(map[string]*ServiceInfo)
//
//	msgMap = make(map[uint32]func([]byte))
//	msgMap[protocol.CmdToken] = S2cCheckRsp
//	msgMap[protocol.CmdStartZone] = S2cStartZone
//	msgMap[protocol.CmdStopZone] = S2cStopZone
//	msgMap[protocol.CmdUpdateHost] = S2cUpdateZoneConfig
//	msgMap[protocol.CmdStartHostZone] = S2cStartHostZones
//	msgMap[protocol.CmdStopHostZone] = S2cStopHostZones
//	msgMap[protocol.CmdNewZone] = S2cNewZone
//	msgMap[protocol.CmdUpdateSvn] = S2cUpdateSvn
//}

func ExecPhpForLogdb() {
	for {
		for k := range logConfs.logPhpArg {
			utils.ExeShell("php", conf.CgPhp, logConfs.logPhpArg[k])
		}
		time.Sleep(time.Minute * 5)
	}
}

func (agent *Agent) CheckProcessStatus(checkShellName string, dstName string) {
	for {
		s := agent.agentServices[dstName]
		if s.Started == false {
			s.Gof = false
			break
		}
		if CheckProcess(s.Tservice, dstName) == false {
			log.Println("check process error ", dstName)
		}
		time.Sleep(time.Minute * 30)
	}
}

//检查进程是否存在
func CheckProcess(tservice int, dstName string) bool {
	checkShellName, ok := CheckProcessName[tservice]
	if !ok {
		log.Println("checkprocess cannt get type shellName", dstName, tservice)
		return false
	}

	check := conf.ProductDir + "/agent/" + checkShellName
	ret, _ := utils.ExeShell("sh", check, dstName)
	s := strings.Replace(string(ret), " ", "", -1)
	if s != "" {
		return false
	}
	return true
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

	var exeErr error
	codeVersion, exeErr = utils.ExeShell("sh", conf.CmdSvnVer, "")
	if exeErr != nil {
		log.Fatal(exeErr)
	}

	os.Mkdir(conf.LocalConfDir, os.ModePerm)
}

func New() *Agent {
	LoadConfig()
	a := Agent{
		agentServices: make(map[string]*ServiceInfo),
		msgMap:        make(map[uint32]func([]byte)),
	}
	logConfs.logdbIP = make(map[string]string)
	logConfs.logPhpArg = make(map[string]string)

	a.msgMap[protocol.CmdToken] = a.S2cCheckRsp
	a.msgMap[protocol.CmdStartZone] = a.S2cStartZone
	a.msgMap[protocol.CmdStopZone] = a.S2cStopZone
	a.msgMap[protocol.CmdUpdateHost] = a.S2cUpdateZoneConfig
	a.msgMap[protocol.CmdStartHostZone] = a.S2cStartHostZones
	a.msgMap[protocol.CmdStopHostZone] = a.S2cStopHostZones
	a.msgMap[protocol.CmdNewZone] = a.S2cNewZone
	a.msgMap[protocol.CmdUpdateSvn] = a.S2cUpdateSvn

	return &a

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

func (agent *Agent) C2sCheckReq() {
	p := protocol.C2sToken{
		Mservice: make(map[string]bool),
	}
	host, hostErr := os.Hostname()
	if hostErr != nil {
		log.Println(":checkReq:", hostErr.Error())
	}

	_, exeErr := utils.ExeShellArgs3("expect", "./synGameConf_expt", conf.RemoteIP, conf.RemoteConfDir, conf.LocalConfDir)
	if exeErr != nil {
		log.Println("Update cannt work!, reason:", exeErr.Error())
	}
	agent.LoadLogFile()

	for k, v := range agent.agentServices {
		p.Mservice[k] = v.Started
	}
	p.Host = host
	p.Token = utils.CreateMd5("cgyx2017")
	p.CodeVersion = codeVersion
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

func (agent *Agent) LoadLogFile() {
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
		logConfs.logdbIP[db.DirName] = db.IP
		logConfs.logPhpArg[db.DirName] = fmt.Sprintf(conf.PhpTemplate, db.IP, db.DirName, "Crontab.dataProcess", db.IP)
		log.Println("LoadLogFile, logs:, ", db.DirName, logConfs.logPhpArg[db.DirName])
		agent.InitServiceStatus(serviceName)
	}
	go ExecPhpForLogdb()
}

//目前只有zone级服务初始化,后面添加登陆、充值等
func (agent *Agent) InitServiceStatus(name string) {
	agent.agentServices[name] = &ServiceInfo{}
	t := utils.AgentServiceType(name, protocol.TserviceReg)
	agent.agentServices[name].Tservice = t
	agent.agentServices[name].Started = CheckProcess(t, name)
	agent.agentServices[name].Gof = false
	agent.agentServices[name].Sname = name

	if agent.agentServices[name].Started {
		go agent.CheckProcessStatus("checkZoneProcess", name)
		agent.agentServices[name].Gof = true
	}
}

func (agent *Agent) StartZone(zone string) int {
	t := agent.agentServices[zone].Tservice
	log.Println("recv start zone, zone:", zone)
	s := CheckProcess(t, zone)
	if s == false {
		utils.ExeShellArgs3("sh", conf.ProductDir+"/cgServer", "start", zone, "")
		for index := 0; index < 6; index++ {
			if s {
				break
			}
			s = CheckProcess(t, zone)
			time.Sleep(time.Second * 5)
		}
	}
	ret := protocol.NotifyDoFail
	agent.agentServices[zone].Started = s
	if s == true {
		if agent.agentServices[zone].Gof == false {
			go agent.CheckProcessStatus("checkZoneProcess", zone)
			agent.agentServices[zone].Gof = true
		}
		ret = protocol.NotifyDoSuc
	}
	agent.S2cZoneState(zone)
	agent.agentServices[zone].Operating = false
	return ret
}

func (agent *Agent) StopZone(zone string) int {
	log.Println("recv start zone, zone:", zone)
	t := agent.agentServices[zone].Tservice

	s := CheckProcess(t, zone)
	if s == false {
		agent.agentServices[zone].Started = false
		agent.agentServices[zone].Gof = false

		return protocol.NotifyDoSuc
	}
	utils.ExeShellArgs2("sh", conf.ProductDir+"/cgServer", "stop", zone)
	s = CheckProcess(t, zone)
	if s {
		return protocol.NotifyDoFail
	}

	agent.agentServices[zone].Started = false
	agent.agentServices[zone].Gof = false
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
	_, exeErr := utils.ExeShellArgs3("expect", "./synGameConf_expt", conf.RemoteIP, conf.RemoteConfDir, conf.LocalConfDir)
	if exeErr != nil {
		log.Println("Update cannt work!, reason:", exeErr.Error())
		r.Do = protocol.NotifyDoFail
	} else {
		r.Do = protocol.NotifyDoSuc
	}
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

	if agent.agentServices[zone].Operating {
		r.Do = protocol.NotifyDoing
		protocol.SendJson(agent.conn, protocol.CmdStartZone, r)
		return
	}
	agent.agentServices[zone].Operating = true
	r.Do = agent.StartZone(zone)
	agent.agentServices[zone].Operating = false
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
	if agent.agentServices[zone].Operating {
		r.Do = protocol.NotifyDoing
		protocol.SendJson(agent.conn, protocol.CmdStopZone, r)
		return
	}
	agent.agentServices[zone].Operating = true
	r.Do = agent.StopZone(zone)
	agent.agentServices[zone].Operating = false
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

	for k := range agent.agentServices {
		z := agent.agentServices[k]
		ret := agent.StartZone(z.Sname)
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
	log.Println("aaaa:", agent.agentServices)
	for k := range agent.agentServices {
		z := agent.agentServices[k]
		ret := agent.StopZone(z.Sname)
		if ret != protocol.NotifyDoSuc {
			r.Do = ret
		}

	}
	protocol.SendJson(agent.conn, protocol.CmdStopHostZone, r)
}

func (agent *Agent) S2cZoneState(zone string) {
	p := protocol.C2sZoneState{
		Zone: zone,
		Open: agent.agentServices[zone].Started,
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
	_, exeErr := utils.ExeShellArgs3("expect", "./synGameConf_expt", conf.RemoteIP, conf.RemoteConfDir, conf.LocalConfDir)
	if exeErr != nil {
		log.Println("Update cannt work!, reason:", exeErr.Error())
		r.Do = protocol.NotifyDoFail
	} else {
		r.Do = protocol.NotifyDoSuc
		agent.InitServiceStatus(p.Name)
	}
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
			codeVersion = r.Result
		}
	}
	protocol.SendJson(agent.conn, protocol.CmdNewZone, r)
}
