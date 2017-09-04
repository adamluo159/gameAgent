package agentClient

import (
	"log"
	"strings"
	"time"

	"github.com/adamluo159/gameAgent/utils"
)

//定时执行php命令处理磁盘上的游戏日志文件
func (agent *Agent) GameLogFileToDB() {
	for {
		for k := range agent.logPhpArg {
			utils.ExeShell("php", conf.CgPhp, agent.logPhpArg[k])
		}
		time.Sleep(time.Minute * 5)
	}
}

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
