package agentServer

import (
	"bytes"
	"encoding/json"
	"log"

	"github.com/adamluo159/gameAgent/protocol"
	"github.com/adamluo159/gameAgent/utils"
)

type AgentMsg struct {
	Cmd  string
	Host string
	Data string
}

func (c *Client) RegCmd() *map[uint32]func(data []byte) {
	return &map[uint32]func(data []byte){
		protocol.CmdToken: c.TokenCheck,
	}
}

func (c *Client) OnMessage() {
	// 消息缓冲
	msgbuf := bytes.NewBuffer(make([]byte, 0, 1024))
	// 数据缓冲
	databuf := make([]byte, 1024)
	// 消息长度
	length := 0

	//注册消息处理
	msgMap := c.RegCmd()

	for {
		// 读取数据
		n, err := (*c.conn).Read(databuf)
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
			//// 消息头
			//if length == 0 && msgbuf.Len() >= 4 {
			//	binary.Read(msgbuf, binary.LittleEndian, &ulength)
			//	length = int(ulength)
			//	// 检查超长消息
			//	if length > 1024 {
			//		log.Printf("Message too length: %d\n", length)
			//		return
			//	}
			//}
			//// 消息体
			//if length > 0 && msgbuf.Len() >= length {
			//	binary.Read(msgbuf, binary.BigEndian, &cmd)
			//	data := msgbuf.Next(length - 4)
			//	length = 0
			//	mfunc := (*msgMap)[cmd]
			//	if mfunc == nil {
			//		log.Printf("cannt find msg handle Client cmd: %d data: %s\n", cmd, string(data))
			//	} else {
			//		mfunc(data)
			//		log.Printf("Client cmd: %d data: %s\n", cmd, string(data))
			//	}
			//} else {
			//	break
			//}
			cmd, data := protocol.UnPacket(&length, msgbuf)
			if cmd <= 0 {
				break
			}
			mfunc := (*msgMap)[cmd]
			if mfunc == nil {
				log.Printf("cannt find msg handle Client cmd: %d data: %s\n", cmd, string(data))
			} else {
				mfunc(data)
				log.Printf("Client cmd: %d data: %s\n", cmd, string(data))
			}

		}
	}
}

func (c *Client) TokenCheck(data []byte) {
	p := protocol.C2sToken{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println("TokenCheck, uncode error: ", string(data))
		return
	}
	if utils.Md5Check(p.Token, "cgyx2017") == false {
		(*c.conn).Close()
		return
	}
	c.host = p.Host
	gserver.clients[p.Host] = c

	r := protocol.S2cToken{
		StaticIp: "192.168.1.1",
		Zones:    make(map[int]string),
	}
	protocol.SendJson(c.conn, protocol.CmdToken, r)
}
