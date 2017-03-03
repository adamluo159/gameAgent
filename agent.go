package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
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

func main() {
	for {
		Conn("192.168.1.52:3300")
		time.Sleep(5 * time.Second)
	}

}

func Conn(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("agent conn fail, addr:", addr)
		return
	}
	defer conn.Close()

	gConn = &conn

	msgMap = make(map[string]func(data string))
	msgMap["checked"] = CheckRsp
	msgMap["start"] = Start
	msgMap["stop"] = Stop
	msgMap["connected"] = CheckReq
	msgMap["update"] = Update

	go Ping()

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

func CheckReq(string) {
	log.Println("ccccc")
	a := AgentMsg{}
	a.Cmd = "token"
	md5Ctx := md5.New()
	md5Ctx.Write([]byte("cgyx2017"))
	cipherStr := md5Ctx.Sum(nil)
	a.Data = hex.EncodeToString(cipherStr)
	host, hostErr := os.Hostname()
	if hostErr != nil {
		fmt.Println(":checkReq:", hostErr.Error())
	}

	a.Host = host
	data, err := json.Marshal(a)
	if err != nil {
		fmt.Println("checkReq: ", err.Error())
		return
	}
	jerr := Send(data)
	if jerr != nil {
		fmt.Println("checkReq:", jerr.Error())
		return
	}
}

func CheckRsp(data string) {
	fmt.Println("recv conn agent rsp, data:", data)
	if data != "OK" {
		// 执行系统命令
		// 第一个参数是命令名称
		// 后面参数可以有多个，命令参数
		cmd := exec.Command("sh", "GameConfig/gitUpdate")
		// 获取输出对象，可以从该对象中读取输出结果
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Println(err.Error())
		}
		// 保证关闭输出流
		defer stdout.Close()
		// 运行命令
		if err := cmd.Start(); err != nil {
			fmt.Println(err.Error())
		}
		// 读取输出结果
		opBytes, err := ioutil.ReadAll(stdout)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(string(opBytes))
	}
}

func Start(data string) {
	fmt.Println("recv start msg, data:", data)
}

func Stop(data string) {
	fmt.Println("recv stop msg, data:", data)
}

func Update(data string) {
	fmt.Println("recv Update msg, data:", data)
}

func Ping() {
	a := AgentMsg{
		Cmd: "ping",
	}
	data, err := json.Marshal(a)
	if err != nil {
		fmt.Println("checkReq: ", err.Error())
		return
	}
	var jerr error
	for {
		log.Println("send ping ...")
		jerr = Send(data)
		if jerr != nil {
			fmt.Println("pingErr:", jerr.Error())
			return
		}

		time.Sleep(10 * time.Second)
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
