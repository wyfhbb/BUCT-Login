package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	baseIP = IPDetect()
	ua     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.81 Safari/537.36"
	client = &http.Client{
		Timeout: 5 * time.Second,
	}
)

// ReloadBaseURL 重新加载基础URL
func ReloadBaseURL() {
	baseIP = IPDetect()
}

// GetChallenge 获取challenge值
func GetChallenge(username, ip string) (string, error) {
	logrus.Debug("getChallenge")

	callback, timestamp := JSONPGenerator()

	reqURL := fmt.Sprintf("http://%s/cgi-bin/get_challenge", baseIP)
	params := url.Values{}
	params.Set("username", username)
	params.Set("ip", ip)
	params.Set("callback", callback)
	params.Set("_", strconv.FormatInt(timestamp, 10))

	fullURL := reqURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", err
	}

	setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	data, err := FilterJSONP(callback, string(body))
	if err != nil {
		return "", err
	}

	if challenge, ok := data["challenge"].(string); ok {
		return challenge, nil
	}

	return "", fmt.Errorf("challenge not found in response")
}

// GetStatus 获取登录状态
func GetStatus() (string, bool, error) {
	logrus.Debug("getStatus")

	callback, timestamp := JSONPGenerator()

	reqURL := fmt.Sprintf("http://%s/cgi-bin/rad_user_info", baseIP)
	params := url.Values{}
	params.Set("callback", callback)
	params.Set("_", strconv.FormatInt(timestamp, 10))

	fullURL := reqURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", false, err
	}

	setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, err
	}

	data, err := FilterJSONP(callback, string(body))
	if err != nil {
		return "", false, err
	}

	logrus.Debug(data)

	if errorMsg, exists := data["error"]; exists && errorMsg == "not_online_error" {
		return "", false, nil
	}

	if onlineIP, exists := data["online_ip"]; exists {
		if ip, ok := onlineIP.(string); ok {
			return ip, true, nil
		}
	}

	return "", false, nil
}

// Login 登录
func Login(username, password, acid, ip string) (map[string]interface{}, error) {
	reqURL := fmt.Sprintf("http://%s/cgi-bin/srun_portal", baseIP)
	callback, timestamp := JSONPGenerator()

	challenge, err := GetChallenge(username, ip)
	if err != nil {
		return nil, err
	}

	msg := fmt.Sprintf(`{"username":"%s","password":"%s","ip":"%s","acid":"20","enc_ver":"srun_bx1"}`, username, password, ip)
	i := Info(msg, challenge)
	hmd5 := Md5(password, challenge)
	passwordMD5 := "{MD5}" + hmd5
	device, platform := GetOS()

	params := url.Values{}
	params.Set("callback", callback)
	params.Set("action", "login")
	params.Set("username", username)
	params.Set("password", passwordMD5)
	params.Set("ac_id", acid)
	params.Set("ip", ip)
	params.Set("chksum", GetChksum(challenge, username, hmd5, acid, ip, 200, 1, i))
	params.Set("info", i)
	params.Set("n", "200")
	params.Set("type", "1")
	params.Set("os", device)
	params.Set("name", platform)
	params.Set("double_stack", "0")
	params.Set("_", strconv.FormatInt(timestamp, 10))

	fullURL := reqURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return FilterJSONP(callback, string(body))
}

// Logout 登出
func Logout(ip, acID string) (map[string]interface{}, error) {
	reqURL := fmt.Sprintf("http://%s/cgi-bin/srun_portal", baseIP)
	callback, timestamp := JSONPGenerator()

	params := url.Values{}
	params.Set("callback", callback)
	params.Set("action", "logout")
	params.Set("ac_id", acID)
	params.Set("ip", ip)
	params.Set("_", strconv.FormatInt(timestamp, 10))

	fullURL := reqURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return FilterJSONP(callback, string(body))
}

// GetUserInfo 获取用户信息
func GetUserInfo() (map[string]interface{}, error) {
	callback, timestamp := JSONPGenerator()
	reqURL := fmt.Sprintf("http://%s/cgi-bin/rad_user_info", baseIP)

	params := url.Values{}
	params.Set("callback", callback)
	params.Set("_", strconv.FormatInt(timestamp, 10))

	fullURL := reqURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{"error": "timeout"}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return FilterJSONP(callback, string(body))
}

// setHeaders 设置请求头
func setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Cookie", "lang=zh-CN; _ga=GA1.3.1108883948.1583300884")
	req.Header.Set("Referer", fmt.Sprintf("http://%s/srun_portal_pc?ac_id=20&theme=basic", baseIP))
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Connection", "keep-alive")
}
