package agentServer

import (
	"bytes"
	"encoding/json"
	"log"

	"github.com/adamluo159/admin-react/server/machine"
	"github.com/adamluo159/gameAgent/protocol"
	"github.com/adamluo159/gameAgent/utils"
)

func (c *Client) RegCmd() *map[uint32]func(data []byte) {
	return &map[uint32]func(data []byte){
		protocol.CmdToken:     c.TokenCheck,
		protocol.CmdStartZone: c.CallBackHandle,
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
			cmd, data := protocol.UnPacket(&length, msgbuf)
			if cmd <= 0 {
				break
			}
			mfunc := (*msgMap)[cmd]
			if mfunc == nil {
				log.Printf("cannt find msg handle Client cmd: %d data: %s\n", cmd, string(data))
			} else {
				log.Printf("Client cmd: %d data: %s\n", cmd, string(data))
				mfunc(data)
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

	m := machine.GetMachineByName(p.Host)
	if m == nil {
		log.Println("TokenCheck cant find machine, host:", p.Host)
		return
	}
	sm := machine.GetMachineByName("master")
	if sm == nil {
		log.Println("TokenCheck cant find staticIp machine, host:", "master")
		return
	}

	r := protocol.S2cToken{
		StaticIp:     m.IP,
		Applications: m.Applications,
	}
	protocol.SendJson(c.conn, protocol.CmdToken, r)
}

func (c *Client) CallBackHandle(data []byte) {
	r := protocol.C2sNotifyDone{}
	err := json.Unmarshal(data, &r)
	if err != nil {
		log.Println("CallBackHandle, uncode error: ", string(data))
		return
	}
	protocol.NotifyWait(r.Req, r)
}
