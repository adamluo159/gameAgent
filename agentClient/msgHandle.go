package agentClient

import (
	"encoding/json"
	"log"

	"github.com/adamluo159/gameAgent/protocol"
	"github.com/adamluo159/gameAgent/utils"
)

func (agent *Agent) CmdReg() {
	agent.msgMap[protocol.CmdToken] = agent.S2cCheckRsp
	agent.msgMap[protocol.CmdStartZone] = agent.S2cStartZone
	agent.msgMap[protocol.CmdStopZone] = agent.S2cStopZone
	agent.msgMap[protocol.CmdUpdateHost] = agent.S2cUpdateZoneConfig
	agent.msgMap[protocol.CmdStartHostZone] = agent.S2cStartHostZones
	agent.msgMap[protocol.CmdStopHostZone] = agent.S2cStopHostZones
	agent.msgMap[protocol.CmdNewZone] = agent.S2cNewZone
	agent.msgMap[protocol.CmdUpdateSvn] = agent.S2cUpdateSvn

}

//同步本机信息(机器名、机器上服务以及起停状态、svn代码版本号)
func (agent *Agent) C2sCheckReq() {
	p := protocol.C2sToken{
		Mservice: make(map[string]bool),
	}

	p.Host = hostName
	p.Token = utils.CreateMd5("cgyx2017")
	p.CodeVersion = SvnVer

	for k, v := range agent.srvs {
		p.Mservice[k] = v.Started
	}

	protocol.SendJson(agent.conn, protocol.CmdToken, &p)
}

//同步回复
func (agent *Agent) S2cCheckRsp(data []byte) {
	r := string(data)
	if r != "OK" {
		log.Fatal("register agentserver callback not ok")
	}
	//p := protocol.S2cToken{}
	//err := json.Unmarshal(data, &p)
	//if err != nil {
	//	log.Println("CheckRsp, uncode error:", string(data), err.Error())
	//	return
	//}
}

//更新zone配置信息
func (agent *Agent) S2cUpdateZoneConfig(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Stop Zone uncode json err, zone:", err.Error())
		return
	}
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoFail,
	}

	log.Println("update zoneConfig, Name:", p.Name, "req:", p.Req)
	UpdateGameConf()

	r.Do = protocol.NotifyDoSuc
	protocol.SendJson(agent.conn, protocol.CmdUpdateHost, r)
}

//启动游戏服
func (agent *Agent) S2cStartZone(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" StartZone uncode json err, zone:", err.Error())
		return
	}
	zone := p.Name
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoFail,
	}

	if agent.srvs[zone].Operating {
		r.Do = protocol.NotifyDoing
		protocol.SendJson(agent.conn, protocol.CmdStartZone, r)
		return
	}
	agent.srvs[zone].Operating = true

	run := StartZone(zone)

	if run {
		agent.srvs[zone].RegularlyCheck = true
		agent.C2sZoneState(zone)
		r.Do = protocol.NotifyDoSuc

	} else {
		r.Do = protocol.NotifyDoFail
	}

	agent.srvs[zone].Started = run
	agent.srvs[zone].Operating = false
	protocol.SendJson(agent.conn, protocol.CmdStartZone, r)
}

//关闭游戏服
func (agent *Agent) S2cStopZone(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Stop Zone uncode json err, zone:", err.Error())
		return
	}
	zone := p.Name
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoFail,
	}
	log.Println("recv stop msg, Name:", zone, "req:", p.Req)
	if agent.srvs[zone].Operating {
		r.Do = protocol.NotifyDoing
		protocol.SendJson(agent.conn, protocol.CmdStopZone, r)
		return
	}
	agent.srvs[zone].Operating = true

	if StopZone(zone) {
		agent.srvs[zone].Started = false
		agent.srvs[zone].RegularlyCheck = false
		r.Do = protocol.NotifyDoSuc
	} else {
		r.Do = protocol.NotifyDoFail
	}

	agent.srvs[zone].Operating = false
	protocol.SendJson(agent.conn, protocol.CmdStopZone, r)
}

//启动机器上所有的游戏服
func (agent *Agent) S2cStartHostZones(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Start hostZones uncode json err, zone:", err.Error())
		return
	}
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoSuc,
	}

	for k, v := range agent.srvs {
		run := StartZone(k)
		v.Started = run
		if run {
			agent.srvs[k].RegularlyCheck = true
			agent.C2sZoneState(k)
		} else {
			r.Do = protocol.NotifyDoFail
		}

	}
	protocol.SendJson(agent.conn, protocol.CmdStartHostZone, r)
}

//关闭机器上所有的游戏服
func (agent *Agent) S2cStopHostZones(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Stop hostZones uncode json err, zone:", err.Error())
		return
	}
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoSuc,
	}
	for k, v := range agent.srvs {
		stop := StopZone(k)
		if stop {
			v.Started = false
			v.RegularlyCheck = false
			agent.C2sZoneState(k)
		} else {
			r.Do = protocol.NotifyDoFail
		}
	}
	protocol.SendJson(agent.conn, protocol.CmdStopHostZone, r)
}

//agent同步游戏状态
func (agent *Agent) C2sZoneState(zone string) {
	p := protocol.C2sZoneState{
		Zone: zone,
		Open: agent.srvs[zone].Started,
	}
	err := protocol.SendJson(agent.conn, protocol.CmdZoneState, p)
	if err != nil {
		log.Println("sysn zone state err, ", err.Error())
	}
}

//新配置游戏服信息同步
func (agent *Agent) S2cNewZone(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Stop Zone uncode json err, zone:", err.Error())
		return
	}
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoSuc,
	}
	log.Println("update zoneConfig, Name:", p.Name, "req:", p.Req)

	UpdateGameConf()
	agent.InitSrv(p.Name)

	protocol.SendJson(agent.conn, protocol.CmdNewZone, r)
}

//更新svn
func (agent *Agent) S2cUpdateSvn(data []byte) {
	p := protocol.S2cNotifyDo{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		log.Println(" Stop Zone uncode json err, zone:", err.Error())
		return
	}
	r := protocol.C2sNotifyDone{
		Req: p.Req,
		Do:  protocol.NotifyDoSuc,
	}

	log.Println("update zoneConfig, Name:", p.Name, "req:", p.Req)

	SvnUp()
	SvnInfo()

	r.Result = SvnVer
	protocol.SendJson(agent.conn, protocol.CmdNewZone, r)
}
