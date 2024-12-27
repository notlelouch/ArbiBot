// internal/exchange/kucoin/auth.go
package kucoin

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io" // Replace io/ioutil with io
	"net/http"
	"strconv"
	"time"
)

// TokenResponse struct remains the same
type TokenResponse struct {
	Code string `json:"code"`
	Data struct {
		Token           string `json:"token"`
		InstanceServers []struct {
			Endpoint     string `json:"endpoint"`
			Protocol     string `json:"protocol"`
			Encrypt      bool   `json:"encrypt"`
			PingInterval int    `json:"pingInterval"`
			PingTimeout  int    `json:"pingTimeout"`
		} `json:"instanceServers"`
	} `json:"data"`
}

func GetToken(apiKey, apiSecret, passphrase string, isPrivate bool) (*TokenResponse, error) {
	timestamp := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	endpoint := "/api/v1/bullet-public"
	if isPrivate {
		endpoint = "/api/v1/bullet-private"
	}

	// Create the signature
	strToSign := timestamp + "POST" + endpoint
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(strToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Make the request
	client := &http.Client{}
	reqBody := []byte(`{}`)
	req, err := http.NewRequest("POST", "https://api.kucoin.com"+endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("KC-API-KEY", apiKey)
	req.Header.Set("KC-API-SIGN", signature)
	req.Header.Set("KC-API-TIMESTAMP", timestamp)
	req.Header.Set("KC-API-PASSPHRASE", passphrase)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Replace ioutil.ReadAll with io.ReadAll
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	if tokenResp.Code != "200000" {
		return nil, fmt.Errorf("failed to get token: %s", tokenResp.Code)
	}

	return &tokenResp, nil
}
