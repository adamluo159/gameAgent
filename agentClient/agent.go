package agentClient

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"encoding/json"

	"fmt"

	"github.com/adamluo159/gameAgent/protocol"
	"github.com/adamluo159/gameAgent/utils"
)

type LogsInfo struct {
	logdbIP   map[string]string
	logPhpArg map[string]string
	StaticIP  string
}

type LogdbConf struct {
	DirName string
	IP      string
}

type ServiceInfo struct {
	Sname     string
	Started   bool
	Gof       bool
	Operating bool
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
	cgServerFile = cgProductDir + "/cgserver"
	cgPhp = cgProductDir + "/php/api/api.php"

	os.Mkdir(localDir, os.ModePerm)
	msgMap = make(map[uint32]func([]byte))
	msgMap[protocol.CmdToken] = CheckRsp
	msgMap[protocol.CmdStartZone] = StartZone
	msgMap[protocol.CmdStopZone] = StopZone
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
		if CheckProcess(checkShellName, dstName) == false {
			log.Println("check process error ", dstName)
		}
		time.Sleep(time.Minute * 30)
	}
}

func CheckProcess(checkShellName string, dstName string) bool {
	check := cgProductDir + "/agent/" + checkShellName
	ret, _ := utils.ExeShell("sh", check, dstName)
	if ret != "" {
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
	p := protocol.C2sToken{}
	host, hostErr := os.Hostname()
	if hostErr != nil {
		log.Println(":checkReq:", hostErr.Error())
	}
	p.Host = host
	p.Token = utils.CreateMd5("cgyx2017")
	protocol.SendJson(gConn, protocol.CmdToken, &p)
}

func CheckRsp(data []byte) {
	p := protocol.S2cToken{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println("CheckRsp, uncode error:", string(data), err.Error())
		return
	}
	logConfs.StaticIP = p.StaticIp
	_, exeErr := utils.ExeShellArgs3("expect", "./synGameConf_expt", connectIP, gConfDir, localDir)
	if exeErr != nil {
		log.Println("Update cannt work!, reason:", exeErr.Error())
	}
	LoadLogFile()
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
	//utils.SetTimerPerHour(ExecPhpForLogdb)
	go ExecPhpForLogdb()
}

//目前只有zone级服务初始化,后面添加登陆、充值等
func InitServiceStatus(name string) {
	agentServices[name] = &ServiceInfo{}
	agentServices[name].Started = CheckProcess("checkZoneProcess", name)
	agentServices[name].Gof = false

	if agentServices[name].Started {
		go CheckProcessStatus("checkZoneProcess", name)
		agentServices[name].Gof = true
	}
}

func StartZone(data []byte) {
	zone := string(data)
	if agentServices[zone].Operating {
		return
	}
	agentServices[zone].Operating = true
	log.Println("recv start zone, zone:", zone)
	s := CheckProcess("checkZoneProcess", zone)
	if s == false {
		utils.ExeShellArgs3("sh", cgServerFile, "start", zone, "")
		for index := 0; index < 6; index++ {
			if s {
				break
			}
			s = CheckProcess("checkZoneProcess", zone)
			time.Sleep(time.Second * 5)
		}
	}
	agentServices[zone].Started = s
	if s == true && agentServices[zone].Gof == false {
		go CheckProcessStatus("checkZoneProcess", zone)
		agentServices[zone].Gof = true
	}
	agentServices[zone].Operating = false
}

func StopZone(data []byte) {
	zone := string(data)
	if agentServices[zone].Operating {
		return
	}
	agentServices[zone].Operating = true
	log.Println("recv stop msg, ", zone)
	utils.ExeShellArgs2("sh", cgServerFile, "stop", zone)
	agentServices[zone].Operating = false
}

func GetSeviceStarted(data []byte) {
	zone := string(data)
	p := protocol.C2sServiceStartStatus{
		Name:    zone,
		Started: agentServices[zone].Started,
	}
	protocol.SendJson(gConn, protocol.CmdServiceStarted, p)
}

//func Ping() {
//	for {
//		log.Println("send ping ...")
//		protocol.Send(gConn, protocol.CmdUpdateHost, "ok")
//		time.Sleep(10 * time.Millisecond)
//	}
//}
