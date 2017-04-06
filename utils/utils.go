package utils

import (
	"crypto/md5"
	"encoding/hex"
	"time"
)

//每整点小时调用
func SetTimerPerHour(F func()) {
	go func() {
		for {
			F()
			now := time.Now()
			next := now.Add(time.Hour)
			next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour(), 0, 0, 0, next.Location())
			t := time.NewTimer(next.Sub(now))
			<-t.C
		}
	}()
}

//Md5校验
func Md5Check(checkStr string, gen string) bool {
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(gen))
	cipherStr := md5Ctx.Sum(nil)
	token := hex.EncodeToString(cipherStr)
	if token != checkStr {
		return false
	}
	return true
}

func CreateMd5(gen string) string {
	md5Ctx := md5.New()
	md5Ctx.Write([]byte("cgyx2017"))
	cipherStr := md5Ctx.Sum(nil)
	return hex.EncodeToString(cipherStr)
}
