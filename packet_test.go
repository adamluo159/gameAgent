package main

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"testing"

	"bytes"

	"github.com/adamluo159/gameAgent/protocol"
)

func TestPacket(t *testing.T) {
	a := protocol.C2sToken{
		Host:  "aa",
		Token: "111",
	}

	s, _ := json.Marshal(a)
	d := protocol.Packet(protocol.CmdToken, s)
	log.Println("packet:", string(s), len(s))

	buff := bytes.NewBuffer(d)
	ulength := uint32(0)

	binary.Read(buff, binary.BigEndian, &ulength)
	log.Println("ret:", string(d), ulength, len(d))

}

func TestUnPacket(t *testing.T) {
	a := protocol.C2sToken{
		Host:  "aa",
		Token: "111",
	}

	s, _ := json.Marshal(a)
	d := protocol.Packet(protocol.CmdToken, s)
	log.Println("Unpacket :", string(s), len(s))

	buff := bytes.NewBuffer(d)

	length := 0
	cmd, data := protocol.UnPacket(&length, buff)
	log.Println("unPack ret:", cmd, string(data), len(data))
}
