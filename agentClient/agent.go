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

type LogsInfo struct {
	logdbIP   map[string]string
	logPhpArg map[string]string
}

type LogdbConf struct {
	DirName string
	IP      string
}

type ServiceInfo struct {
	Sname     string
	Started   bool //游戏区服是否已启动
	Gof       bool //是否开启定时检查进程功能
	Operating bool
	Tservice  int
}

var (
	gConn         *net.Conn
	msgMap        map[uint32]func([]byte)
	agentServices map[string]*ServiceInfo //agent起的服务状态(起/停)
	gConfDir      string
	connectIP     string
	hostName      string
	localDir      string
	cgProductDir  string
	cgPhp         string
	cgServerFile  string
	phpTemplate   string = "logdb=%s&logdir=%s&method=%s&sdb=%s"
	logConfs      LogsInfo

	CheckProcessName = map[int]string{
		protocol.Tzone: "checkZoneProcess",
	}
)

func RegCmd() {
	logConfs.logdbIP = make(map[string]string)
	logConfs.logPhpArg = make(map[string]string)
	agentServices = make(map[string]*ServiceInfo)

	hostName, _ = os.Hostname()
	if hostName == "" {
		log.Println("cannt get machine hostname")
	}

	gConfDir = os.Getenv("HOME") + "/gConf/" + hostName
	localDir = os.Getenv("HOME") + "/GameConfig/"
	cgProductDir = os.Getenv("HOME") + "/product/server/"
	cgServerFile = cgProductDir + "/cgServer"
	cgPhp = cgProductDir + "/php/api/api.php"

	os.Mkdir(localDir, os.ModePerm)
	msgMap = make(map[uint32]func([]byte))
	msgMap[protocol.CmdToken] = C2sCheckRsp
	msgMap[protocol.CmdStartZone] = C2sStartZone
	msgMap[protocol.CmdStopZone] = C2sStopZone
	msgMap[protocol.CmdUpdateHost] = UpdateZoneConfig
	msgMap[protocol.CmdStartHostZone] = C2sStartHostZones
	msgMap[protocol.CmdStopHostZone] = C2sStopHostZones
}

func ExecPhpForLogdb() {
	for {
		for k := range logConfs.logPhpArg {
			utils.ExeShell("php", cgPhp, logConfs.logPhpArg[k])
		}
		time.Sleep(time.Minute * 5)
	}
}

func CheckProcessStatus(checkShellName string, dstName string) {
	for {
		s := agentServices[dstName]
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

	check := cgProductDir + "/agent/" + checkShellName
	ret, _ := utils.ExeShell("sh", check, dstName)
	s := strings.Replace(string(ret), " ", "", -1)
	if s != "" {
		return false
	}
	return true
}

func New() {
	RegCmd()

	ip, err := ioutil.ReadFile("./ConnectAddress")
	if err != nil {
		log.Fatal("agent must read ConnectAddress File and get connect IP")
	}
	connectIP = strings.Replace(string(ip), "\n", "", -1)
	connStr := connectIP + ":3300"

	for {
		Conn(connStr)
		time.Sleep(5 * time.Second)
	}
}

func Conn(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Println("agent conn fail, addr:", addr)
		return
	}
	defer conn.Close()

	gConn = &conn
	CheckReq()

	//只在内网跑，所有不加ping了
	//go Ping()

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
			mfunc := msgMap[cmd]
			if mfunc == nil {
				log.Printf("cannt find msg handle server cmd: %d data: %s\n", cmd, string(data))
			} else {
				mfunc(data)
				log.Printf("server cmd: %d data: %s\n", cmd, string(data))
			}
		}
	}

}

func CheckReq() {
	p := protocol.C2sToken{
		Mservice: make(map[string]bool),
	}
	host, hostErr := os.Hostname()
	if hostErr != nil {
		log.Println(":checkReq:", hostErr.Error())
	}

	_, exeErr := utils.ExeShellArgs3("expect", "./synGameConf_expt", connectIP, gConfDir, localDir)
	if exeErr != nil {
		log.Println("Update cannt work!, reason:", exeErr.Error())
	}
	LoadLogFile()

	for k, v := range agentServices {
		p.Mservice[k] = v.Started
	}
	p.Host = host
	p.Token = utils.CreateMd5("cgyx2017")
	protocol.SendJson(gConn, protocol.CmdToken, &p)
}

func C2sCheckRsp(data []byte) {
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

func LoadLogFile() {
	hostDir := localDir + hostName
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
		logConfs.logPhpArg[db.DirName] = fmt.Sprintf(phpTemplate, db.IP, db.DirName, "Crontab.dataProcess", db.IP)
		log.Println("LoadLogFile, logs:, ", db.DirName, logConfs.logPhpArg[db.DirName])
		InitServiceStatus(serviceName)
	}
}

//目前只有zone级服务初始化,后面添加登陆、充值等
func InitServiceStatus(name string) {
	agentServices[name] = &ServiceInfo{}
	t := utils.AgentServiceType(name, protocol.TserviceReg)
	agentServices[name].Tservice = t
	agentServices[name].Started = CheckProcess(t, name)
	agentServices[name].Gof = false
	agentServices[name].Sname = name

	if agentServices[name].Started {
		go CheckProcessStatus("checkZoneProcess", name)
		agentServices[name].Gof = true
	}
}

func StartZone(zone string) int {
	t := agentServices[zone].Tservice
	log.Println("recv start zone, zone:", zone)
	s := CheckProcess(t, zone)
	if s == false {
		utils.ExeShellArgs3("sh", cgServerFile, "start", zone, "")
		for index := 0; index < 6; index++ {
			if s {
				break
			}
			s = CheckProcess(t, zone)
			time.Sleep(time.Second * 5)
		}
	}
	ret := protocol.NotifyDoFail
	agentServices[zone].Started = s
	if s == true {
		if agentServices[zone].Gof == false {
			go CheckProcessStatus("checkZoneProcess", zone)
			agentServices[zone].Gof = true
		}
		ret = protocol.NotifyDoSuc
	}
	C2sZoneState(zone)
	agentServices[zone].Operating = false
	return ret
}

func StopZone(zone string) int {
	log.Println("recv start zone, zone:", zone)
	t := agentServices[zone].Tservice

	s := CheckProcess(t, zone)
	if s == false {
		agentServices[zone].Started = false
		agentServices[zone].Gof = false

		return protocol.NotifyDoSuc
	}
	utils.ExeShellArgs2("sh", cgServerFile, "stop", zone)
	s = CheckProcess(t, zone)
	if s {
		return protocol.NotifyDoFail
	}

	agentServices[zone].Started = false
	agentServices[zone].Gof = false
	C2sZoneState(zone)
	return protocol.NotifyDoSuc
}

func UpdateZoneConfig(data []byte) {
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
	_, exeErr := utils.ExeShellArgs3("expect", "./synGameConf_expt", connectIP, gConfDir, localDir)
	if exeErr != nil {
		log.Println("Update cannt work!, reason:", exeErr.Error())
		r.Do = protocol.NotifyDoFail
	} else {
		r.Do = protocol.NotifyDoSuc
	}
	protocol.SendJson(gConn, protocol.CmdUpdateHost, r)
}

func C2sStartZone(data []byte) {
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

	if agentServices[zone].Operating {
		r.Do = protocol.NotifyDoing
		protocol.SendJson(gConn, protocol.CmdStartZone, r)
		return
	}
	agentServices[zone].Operating = true
	r.Do = StartZone(zone)
	agentServices[zone].Operating = false
	protocol.SendJson(gConn, protocol.CmdStartZone, r)
}

func C2sStopZone(data []byte) {
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
	if agentServices[zone].Operating {
		r.Do = protocol.NotifyDoing
		protocol.SendJson(gConn, protocol.CmdStopZone, r)
		return
	}
	agentServices[zone].Operating = true
	r.Do = StopZone(zone)
	agentServices[zone].Operating = false
	protocol.SendJson(gConn, protocol.CmdStopZone, r)
}

func C2sStartHostZones(data []byte) {
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

	for k := range agentServices {
		z := agentServices[k]
		ret := StartZone(z.Sname)
		if ret != protocol.NotifyDoSuc {
			r.Do = ret
		}
	}
	protocol.SendJson(gConn, protocol.CmdStartHostZone, r)
}

func C2sStopHostZones(data []byte) {
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
	log.Println("aaaa:", agentServices)
	for k := range agentServices {
		z := agentServices[k]
		ret := StopZone(z.Sname)
		if ret != protocol.NotifyDoSuc {
			r.Do = ret
		}

	}
	protocol.SendJson(gConn, protocol.CmdStopHostZone, r)
}

func C2sZoneState(zone string) {
	p := protocol.C2sZoneState{
		Zone: zone,
		Open: agentServices[zone].Started,
	}
	err := protocol.SendJson(gConn, protocol.CmdZoneState, p)
	if err != nil {
		log.Println("sysn zone state err, ", err.Error())
	}
}
