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
		Sname          string //服务名
		Started        bool   //游戏区服是否已启动
		RegularlyCheck bool   //是否开启定时检查进程功能
		Operating      bool
	}

	Agent struct {
		conn      *net.Conn
		msgMap    map[uint32]func([]byte)
		srvs      map[string]*ServiceInfo //后台配置该机器所有的服务信息
		logdbIP   map[string]string       //日志的logdbIP集合
		logPhpArg map[string]string       //php执行参数集合
	}

	Conf struct {
		ConAddress string

		RemoteIP      string //远端游戏配置仓库地址
		RemoteConfDir string //远端游戏配置仓库目录
		LocalConfDir  string //本地游戏配置地址
		ProductDir    string //本地游戏目录

		CgPhp       string //php执行目录
		PhpTemplate string //php执行模板

		CmdSvnVer string //svn版本更新执行串
		CmdSvnUp  string //svn更新
	}

	LogdbConf struct {
		DirName string
		IP      string
	}
)

var (
	conf Conf //agent配置信息

	hostName string //主机名
	SvnVer   string //svn版本号
)

func StartZone(zone string) bool {
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
	return run
}

func StopZone(zone string) bool {
	utils.ExeShellArgs2("sh", conf.ProductDir+"/cgServer", "stop", zone)
	run := CheckProcess(zone)
	return !run
}

func SvnInfo() {
	ver, err := utils.ExeShell("sh", conf.CmdSvnVer, "")
	if err != nil {
		log.Fatal("update svn ", err)
	}
	SvnVer = ver
}

func SvnUp() {
	_, upErr := utils.ExeShell("sh", conf.CmdSvnUp, "")
	if upErr != nil {
		log.Fatal("svn up,", upErr)
	}
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

	SvnInfo()
	UpdateGameConf()
	os.Mkdir(conf.LocalConfDir, os.ModePerm)
}

//检查进程是否存在
func CheckProcess(dstName string) bool {
	check := conf.ProductDir + "/agent/" + "checkZoneProcess"
	ret, _ := utils.ExeShell("sh", check, dstName)
	s := strings.Replace(string(ret), " ", "", -1)
	if s != "" {
		return false
	}
	return true
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
	go a.RegularlyCheckProcess()

	a.CmdReg()
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

//定时执行php命令处理磁盘上的游戏日志文件
func (agent *Agent) GameLogFileToDB() {
	for {
		for k := range agent.logPhpArg {
			utils.ExeShell("php", conf.CgPhp, agent.logPhpArg[k])
		}
		time.Sleep(time.Minute * 5)
	}
}

//定时检查已启动的进程是否现在存在
func (agent *Agent) RegularlyCheckProcess() {
	for {
		for k, v := range agent.srvs {
			if v.RegularlyCheck && CheckProcess(k) == false {
				log.Println("check process error ", k)
			}
		}

		time.Sleep(time.Minute * 30)
	}
}
