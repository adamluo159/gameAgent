package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

//const map[string]func(data string) msgMap
type AgentMsg struct {
	Cmd  string
	Host string
	Data string
}

var msgMap map[string]func(data string)
var gConn *net.Conn

var configDir string
var connectIP string
var hostName string
var hostConfigDir string
var cgServerFile string

func main() {
	ip, err := ioutil.ReadFile("./ConnectAddress")
	if err != nil {
		log.Fatal("agent must read ConnectAddress File and get connect IP")
	}
	hostName, err = os.Hostname()
	if err != nil {
		log.Println("cannt get machine hostname", err.Error())
	}
	connectIP = strings.Replace(string(ip), "\n", "", -1)
	connStr := connectIP + ":3300"
	configDir = os.Getenv("HOME") + "/GameConfig"
	hostConfigDir = configDir + "/" + hostName
	cgServerFile = os.Getenv("HOME") + "/product/server/cgServer"

	os.Mkdir(configDir, os.ModePerm)
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

	msgMap = make(map[string]func(data string))
	msgMap["checked"] = CheckRsp
	msgMap["start"] = Start
	msgMap["stop"] = Stop
	msgMap["update"] = Update

	CheckReq()

	//只在内网跑，所有不加ping了
	//go Ping()

	buffer := make([]byte, 1024)
	for {
		reader := bufio.NewReader(conn)
		len, err := reader.Read(buffer)
		if err != nil {
			log.Println("msg error:", err.Error())
			return
		}
		dataLength := binary.LittleEndian.Uint32(buffer)
		if dataLength <= 0 || dataLength > 1020 {
			continue
		}
		a := AgentMsg{}
		json.Unmarshal(buffer[4:dataLength+4], &a)
		log.Println("recv agentserver msg, msg: ", a, dataLength, len)
		msgMap[a.Cmd](a.Data)
	}

}

func CheckReq() {
	log.Println("ccccc")
	a := AgentMsg{}
	a.Cmd = "token"
	md5Ctx := md5.New()
	md5Ctx.Write([]byte("cgyx2017"))
	cipherStr := md5Ctx.Sum(nil)
	a.Data = hex.EncodeToString(cipherStr)
	host, hostErr := os.Hostname()
	if hostErr != nil {
		log.Println(":checkReq:", hostErr.Error())
	}

	a.Host = host
	data, err := json.Marshal(a)
	if err != nil {
		log.Println("checkReq: ", err.Error())
		return
	}
	jerr := Send(data)
	if jerr != nil {
		log.Println("checkReq:", jerr.Error())
		return
	}
}

func CheckRsp(data string) {
	log.Println("recv conn agent rsp, data:", data)
	if data == "OK" {
		//Update("CheckRsp")
	}
}

func Start(data string) {
	log.Println("recv start msg, data:", data)
	ExeShellUseArg3("sh", cgServerFile, "start", data, "")
}

func Stop(data string) {
	log.Println("recv stop msg, data:", data)
	ExeShellUseArg3("sh", cgServerFile, "stop", data, "")
}

func Update(data string) {
	log.Println("recv Update msg, data:", data)
	exeErr := ExeShellUseArg3("expect", "./synGameConf_expt", connectIP, hostConfigDir, configDir+"/")
	if exeErr != nil {
		log.Println("Update cannt work!, reason:", exeErr.Error())
	}
}

func Ping() {
	a := AgentMsg{
		Cmd: "ping",
	}
	data, err := json.Marshal(a)
	if err != nil {
		log.Println("checkReq: ", err.Error())
		return
	}
	var jerr error
	for {
		log.Println("send ping ...")
		jerr = Send(data)
		if jerr != nil {
			log.Println("pingErr:", jerr.Error())
			return
		}

		time.Sleep(60 * time.Second)
	}
}

func Send(data []byte) error {
	lenData := (uint32)(len(data))
	s := make([]byte, 4)
	binary.LittleEndian.PutUint32(s, lenData)
	buff := bytes.NewBuffer(s)
	buff.Write(data)

	empty := make([]byte, 1024-buff.Len())
	buff.Write(empty)

	_, err := (*gConn).Write(buff.Bytes())
	log.Println("send msg:", len(buff.Bytes()), lenData, string(buff.Bytes()))
	return err
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
