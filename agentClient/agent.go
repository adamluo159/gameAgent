package agentClient

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
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

var msgMap map[uint32]func([]byte)
var gConn *net.Conn

var (
	gConfDir     string
	connectIP    string
	hostName     string
	localDir     string
	cgProductDir string
	cgPhp        string
	phpTemplate  string = "logdb=$s&logdir=%s&method=%s&sdb=%s"

	logConfs LogsInfo
)

func RegCmd() {
	logConfs.logdbIP = make(map[string]string)
	logConfs.logPhpArg = make(map[string]string)

	hostName, _ = os.Hostname()
	if hostName == "" {
		log.Println("cannt get machine hostname")
	}
	gConfDir = os.Getenv("HOME") + "/gConf/" + hostName
	localDir = os.Getenv("HOME") + "/GameConfig/"
	cgProductDir = os.Getenv("HOME") + "/product/server/"
	cgPhp = cgProductDir + "/php/api/api.php"

	os.Mkdir(localDir, os.ModePerm)
	msgMap = make(map[uint32]func([]byte))
	msgMap[protocol.CmdToken] = CheckRsp
}

func ExecPhpForLogdb() {
	for k := range logConfs.logPhpArg {
		log.Println("begin exec php, logdata to logdb, name:", k)
		ExeShell("php", cgPhp, logConfs.logPhpArg[k])
	}
}

func New() {
	RegCmd()

	ip, err := ioutil.ReadFile("./ConnectAddress")
	if err != nil {
		log.Fatal("agent must read ConnectAddress File and get connect IP")
	}
	connectIP = strings.Replace(string(ip), "\n", "", -1)
	connStr := connectIP + ":3300"

	utils.SetTimerPerHour(ExecPhpForLogdb)
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
	exeErr := ExeShellUseArg3("expect", "./synGameConf_expt", connectIP, gConfDir, localDir)
	if exeErr != nil {
		log.Println("Update cannt work!, reason:", exeErr.Error())
	}
	LoadLogFile()
}

func LoadLogFile() {
	hostDir := localDir + hostName
	dir, err := ioutil.ReadDir(localDir)
	if err != nil {
		log.Println("LoadLogFile, read dir err, ", err.Error())
	}
	for index := 0; index < len(dir); index++ {
		file := hostDir + "/" + dir[index].Name() + "/logdbconf"
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
		logConfs.logPhpArg[db.DirName] = fmt.Sprintf(phpTemplate, db.IP, db.DirName, "Crontab.dataProcess", logConfs.StaticIP)
	}
}

func Start(data []byte) {
	log.Println("recv start msg, data:", data)
	//ExeShellUseArg3("sh", cgServerFile, "start", data, "")
}

func Stop(data []byte) {
	log.Println("recv stop msg, data:", data)
	//ExeShellUseArg3("sh", cgServerFile, "stop", data, "")
}

func Ping() {
	for {
		log.Println("send ping ...")
		protocol.Send(gConn, protocol.CmdUpdateHost, "ok")
		time.Sleep(10 * time.Millisecond)
	}
}

func ExeShell(syscmd string, dir string, args string) error {
	log.Println("begin execute shell.....", syscmd, dir, "--", args)
	// 执行系统命令
	// 第一个参数是命令名称
	// 后面参数可以有多个，命令参数
	cmd := exec.Command(syscmd, dir, args) //"GameConfig/gitCommit", "zoneo")
	// 获取输出对象，可以从该对象中读取输出结果
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
		return err
	}
	// 保证关闭输出流
	defer stdout.Close()
	// 运行命令
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
		return err
	}
	// 读取输出结果
	opBytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Fatal(err)
		return err
	}
	e := cmd.Wait()
	if e != nil {
		log.Println("Exeshell error:", e.Error())
	}
	log.Println(string(opBytes))
	return nil
}

func ExeShellUseArg3(syscmd string, dir string, arg1 string, arg2 string, arg3 string) error {
	log.Println("begin execute shell.....", syscmd, dir, "--", arg1, arg2)
	// 执行系统命令
	// 第一个参数是命令名称
	// 后面参数可以有多个，命令参数
	cmd := exec.Command(syscmd, dir, arg1, arg2, arg3) //"GameConfig/gitCommit", "zoneo")
	// 获取输出对象，可以从该对象中读取输出结果
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
		return err
	}
	// 保证关闭输出流
	defer stdout.Close()
	// 运行命令
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
		return err
	}
	// 读取输出结果
	opBytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Fatal(err)
		return err
	}
	e := cmd.Wait()
	if e != nil {
		log.Println("Exeshell error:", e.Error())
	}
	log.Println(string(opBytes))
	return nil
}
