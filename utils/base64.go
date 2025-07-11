package utils

import (
	"fmt"
	"os"
)

const (
	padChar = "="
	alpha   = "LVoJPiCN2R8G90yg+hmFHuacZ1OWMnrsSTXkYpUq/3dlbfKwv6xztjI7DeBE45QA"
)

// getByte 获取字符串中指定位置的字节值
func getByte(s string, i int) int {
	if i >= len(s) {
		return 0
	}
	x := int(s[i])
	if x > 255 {
		fmt.Println("INVALID_CHARACTER_ERR: DOM Exception 5")
		os.Exit(0)
	}
	return x
}

// GetBase64 将字符串转换为base64编码
func GetBase64(s string) string {
	if len(s) == 0 {
		return s
	}

	var result []string
	imax := len(s) - (len(s) % 3)

	// 处理完整的3字节组
	for i := 0; i < imax; i += 3 {
		b10 := (getByte(s, i) << 16) | (getByte(s, i+1) << 8) | getByte(s, i+2)
		result = append(result, string(alpha[b10>>18]))
		result = append(result, string(alpha[(b10>>12)&63]))
		result = append(result, string(alpha[(b10>>6)&63]))
		result = append(result, string(alpha[b10&63]))
	}

	// 处理剩余字节
	remaining := len(s) - imax
	if remaining != 0 {
		if remaining == 1 {
			b10 := getByte(s, imax) << 16
			result = append(result, string(alpha[b10>>18])+string(alpha[(b10>>12)&63])+padChar+padChar)
		} else {
			b10 := (getByte(s, imax) << 16) | (getByte(s, imax+1) << 8)
			result = append(result, string(alpha[b10>>18])+string(alpha[(b10>>12)&63])+string(alpha[(b10>>6)&63])+padChar)
		}
	}

	return joinStrings(result)
}

// joinStrings 连接字符串数组
func joinStrings(strs []string) string {
	var result string
	for _, s := range strs {
		result += s
	}
	return result
}
