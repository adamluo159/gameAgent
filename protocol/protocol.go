package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"log"
	"time"

	"errors"
	"fmt"
	"net"
)

var (
	gReqIndex    int                      = 1 //同步消息序号
	indexChanMap map[int]chan interface{} = make(map[int]chan interface{})
)

const (
	HSize int = 4

	//agent反馈的状态
	NotifyDoFail int = 1
	NotifyDoSuc  int = 2
	NotifyDoing  int = 3
)

const (
	CmdNone           uint32 = 0
	CmdToken          uint32 = 1 //token验证
	CmdStartZone      uint32 = 2 //区服启动
	CmdStopZone       uint32 = 3 //区服停
	CmdServiceStarted uint32 = 4 //获取服务是否启动
	CmdUpdateHost     uint32 = 5 //机器配置更新
)

type C2sToken struct {
	Token string
	Host  string
}

type S2cToken struct {
	StaticIp     string //统计后台ip
	Applications []string
}

type C2sServiceStartStatus struct {
	Name    string
	Started bool
}

type S2cNotifyDo struct {
	Name string
	Req  int
}
type C2sNotifyDone struct {
	Req int
	Do  int
}

func GetReqIndex() int {
	gReqIndex++
	return gReqIndex
}

func NotifyWait(req int, v interface{}) {
	log.Println("notify wait , req:", req, indexChanMap)
	if c, ok := indexChanMap[req]; ok {
		c <- v
		delete(indexChanMap, req)
	}
}
func WaitCallBack(req int, reply interface{}) error {
	log.Println("wait call back msg, req:", req, indexChanMap)
	ch := make(chan interface{})
	indexChanMap[req] = ch
	t := time.NewTimer(time.Second * 30)
	select {
	case reply = <-ch:
		log.Println("wait callback get reply:", reply)
	case <-t.C:
		delete(indexChanMap, req)
		log.Println("wait cb overtime in 30 second, req:", req)
		return errors.New(fmt.Sprintf("wait cb overtime in 30 second, req:%d", req))
	}
	return nil
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

func Send(conn *net.Conn, cmd uint32, s string) error {
	if conn == nil {
		return errors.New(fmt.Sprintf("send msg cmd:%d, conn pointer is nil", cmd, conn))
	}

	b := Packet(cmd, []byte(s))
	_, err := (*conn).Write(b)
	if err != nil {
		return err
	}
	log.Println("send msg:", len(b), cmd, s)
	return nil
}

func SendJson(conn *net.Conn, cmd uint32, v interface{}) error {
	if conn == nil {
		return errors.New(fmt.Sprintf("sendjson send msg cmd:%d, conn pointer is nil", cmd, conn))
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	jsonBytes := Packet(cmd, data)
	_, jerr := (*conn).Write(jsonBytes)
	if jerr != nil {
		return jerr
	}
	log.Println("send msg:", cmd, len(jsonBytes), string(jsonBytes))
	return nil
}
