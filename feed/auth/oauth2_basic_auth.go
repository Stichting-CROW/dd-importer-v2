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

type OauthCredentialsBasicAuth struct {
	ExpireTime     time.Time
	TokenURL       string
	AccessToken    string
	OauthTokenBody map[string]interface{}
}

func (o *OauthCredentialsBasicAuth) GetAccessToken() string {
	if time.Now().After(o.ExpireTime) {
		log.Printf("Get new accesstoken for basic auth %s", o.TokenURL)
		o.refreshToken()
	}
	return o.AccessToken
}

func (o *OauthCredentialsBasicAuth) refreshToken() {
	log.Printf("Refresh token for basic auth %s", o.TokenURL)
	params := url.Values{}
	params.Set("grant_type", "client_credentials")
	if o.OauthTokenBody["scope"] != nil {
		params.Set("scope", o.OauthTokenBody["scope"].(string))
	}
	request_params := bytes.NewBufferString(params.Encode())

	req, err := http.NewRequest("POST", o.TokenURL, request_params)
	req.SetBasicAuth(o.OauthTokenBody["client_id"].(string), o.OauthTokenBody["client_secret"].(string))
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
