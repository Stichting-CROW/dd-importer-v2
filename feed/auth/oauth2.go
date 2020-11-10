package auth

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type OauthCredentials struct {
	ExpireTime     time.Time
	TokenURL       string
	AccessToken    string
	OauthTokenBody interface{}
}

func (o *OauthCredentials) GetAccessToken() string {
	if time.Now().After(o.ExpireTime) {
		o.refreshToken()
	}
	return o.AccessToken
}

type OAuthResult struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func (o *OauthCredentials) refreshToken() {
	jsonValue, err := json.Marshal(o.OauthTokenBody)
	if err != nil {
		log.Print(err)
	}
	req, err := http.NewRequest("POST", o.TokenURL, bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var result OAuthResult
	json.Unmarshal(body, &result)
	o.AccessToken = result.AccessToken
	o.ExpireTime = time.Now().Add(time.Second * time.Duration(result.ExpiresIn))
}
