package auth

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

type OauthCredentials struct {
	ExpireTime     time.Time
	TokenURL       string
	AccessToken    string
	OauthTokenBody map[string]interface{}
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
	log.Print("Refresh token")
	params := url.Values{}
	params.Set("client_id", o.OauthTokenBody["client_id"].(string))
	params.Set("client_secret", o.OauthTokenBody["client_secret"].(string))
	params.Set("grant_type", o.OauthTokenBody["grant_type"].(string))
	params.Set("scope", o.OauthTokenBody["scope"].(string))
	request_params := bytes.NewBufferString(params.Encode())

	req, err := http.NewRequest("POST", o.TokenURL, request_params)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
	o.ExpireTime = time.Now().Add(time.Second*time.Duration(result.ExpiresIn) - time.Second*5)
}
