package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"log"

	"net"
)

const (
	HSize int = 4
)

const (
	CmdNone       uint32 = 0
	CmdToken      uint32 = 1 //token验证
	CmdStartZone  uint32 = 2 //区服启动
	CmdStopZone   uint32 = 3 //区服停止
	CmdUpdateHost uint32 = 4 //机器配置更新
)

type C2sToken struct {
	Token string
	Host  string
}

type S2cToken struct {
	StaticIp     string //统计后台ip
	Applications []string
}

func Packet(cmd uint32, data []byte) []byte {
	dataLen := len(data)
	HeadLen := HSize * 2
	packetlen := dataLen + HeadLen

	var buf = make([]byte, packetlen)
	binary.BigEndian.PutUint32(buf[:HSize], uint32(dataLen+HSize))
	binary.BigEndian.PutUint32(buf[HSize:HeadLen], cmd)
	copy(buf[HeadLen:], data)
	return buf
}

func UnPacket(length *int, msgbuf *bytes.Buffer) (uint32, []byte) {
	cmd := uint32(CmdNone)
	ulength := uint32(0)

	// 消息头
	if *length == 0 && msgbuf.Len() >= 4 {
		binary.Read(msgbuf, binary.BigEndian, &ulength)
		*length = int(ulength)
		// 检查超长消息
		if *length > 1024 {
			log.Printf("Message too length: %d\n", length)
			return 0, nil
		}
	}
	// 消息体
	if *length <= 0 || msgbuf.Len() < *length {
		//log.Printf("Message, len:%d, bufLen:%d", *length, msgbuf.Len())
		return 0, nil
	}

	binary.Read(msgbuf, binary.BigEndian, &cmd)
	*length = *length - HSize

	data := msgbuf.Next(*length)
	*length = 0

	return cmd, data
}

func Send(conn *net.Conn, cmd uint32, s string) {
	if conn == nil {
		log.Printf("send msg cmd:%d, conn pointer is nil", cmd, conn)
		return
	}

	b := Packet(cmd, []byte(s))
	_, err := (*conn).Write(b)
	if err != nil {
		log.Printf("send msg error, cmd:%d, s:%s, err:%s", cmd, s, err.Error())
	} else {
		log.Println("send msg:", len(b), cmd, s)
	}

}

func SendJson(conn *net.Conn, cmd uint32, v interface{}) {
	if conn == nil {
		log.Printf("PacketJson error, cmd:%d, conn pointer is nil", cmd, conn)
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("PacketJson error, cmd:%d, err:%s ", cmd, err.Error())
		return
	}
	jsonBytes := Packet(cmd, data)
	_, jerr := (*conn).Write(jsonBytes)
	if jerr != nil {
		log.Printf("send msg error, cmd:%d, v:%s, err:%s", cmd, v, jerr.Error())
	} else {
		log.Println("send msg:", cmd, len(jsonBytes), string(jsonBytes))
	}
}
