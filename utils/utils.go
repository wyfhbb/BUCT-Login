package utils

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const jQueryVersion = "1.12.4"

// JSONPTitleGenerator 生成JSON回调函数名
func JSONPTitleGenerator() string {
	title := fmt.Sprintf("%.16f%d%d", rand.Float64(), rand.Intn(10), rand.Intn(10))
	title = "jQuery" + strings.ReplaceAll(jQueryVersion+title, ".", "")
	return title
}

// TimeStampGenerator 生成时间戳
func TimeStampGenerator() int64 {
	return time.Now().UnixMilli()
}

// JSONPGenerator 生成JSONP参数
func JSONPGenerator() (string, int64) {
	callback := JSONPTitleGenerator() + "_" + strconv.FormatInt(TimeStampGenerator(), 10)
	timestamp := TimeStampGenerator() + 1
	return callback, timestamp
}

// IPDetect 检测IP地址
func IPDetect() string {
	list := []string{"202.4.130.95", "202.4.130.82"}
	return list[0]
}

// FilterJSONP 过滤JSONP响应
func FilterJSONP(jsonpTitle, data string) (map[string]interface{}, error) {
	// 移除JSONP的封装
	cleanData := strings.TrimPrefix(data, jsonpTitle)
	cleanData = strings.TrimPrefix(cleanData, "(")
	cleanData = strings.TrimSuffix(cleanData, ")")

	var result map[string]interface{}
	err := json.Unmarshal([]byte(cleanData), &result)
	return result, err
}

// GetChksum 计算校验和
func GetChksum(token, username, hmd5, acID, ip string, n, ltype int, i string) string {
	chkstr := token + username
	chkstr += token + hmd5
	chkstr += token + acID
	chkstr += token + ip
	chkstr += token + strconv.Itoa(n)
	chkstr += token + strconv.Itoa(ltype)
	chkstr += token + i
	return Sha1(chkstr)
}

// Md5 计算MD5哈希
func Md5(password, challenge string) string {
	h := hmac.New(md5.New, []byte(challenge))
	h.Write([]byte(password))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Sha1 计算SHA1哈希
func Sha1(v string) string {
	h := sha1.New()
	h.Write([]byte(v))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Info 生成info参数
func Info(params, challenge string) string {
	encoded := XEncode(params, challenge)
	base64Encoded := GetBase64(encoded)
	return "{SRBX1}" + base64Encoded
}

// GetOS 获取操作系统信息
func GetOS() (string, string) {
	return "Windows 95", "Windows"
}
