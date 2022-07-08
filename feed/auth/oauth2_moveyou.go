package auth

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type OauthCredentialsMoveyou struct {
	ExpireTime     time.Time
	TokenURL       string
	AccessToken    string
	OauthTokenBody map[string]interface{}
}

func (o *OauthCredentialsMoveyou) GetAccessToken() string {
	log.Print("Check accessToken MOVEYOU")
	log.Print(o.ExpireTime)
	log.Print(time.Now())
	log.Print(o.AccessToken)
	if time.Now().After(o.ExpireTime) {
		o.refreshToken()
		log.Print(o.ExpireTime)
		log.Print(o.AccessToken)
	}
	return o.AccessToken
}

func (o *OauthCredentialsMoveyou) refreshToken() {
	values := map[string]string{
		"audience":      "https://tomp.goabout.com",
		"client_id":     o.OauthTokenBody["client_id"].(string),
		"client_secret": o.OauthTokenBody["client_secret"].(string),
		"grant_type":    o.OauthTokenBody["grant_type"].(string),
	}

	jsonValue, _ := json.Marshal(values)

	resp, err := http.Post(o.TokenURL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Print(err)
		return
	}

	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var result OAuthResult
	json.Unmarshal(body, &result)
	o.AccessToken = result.AccessToken
	o.ExpireTime = time.Now().Add(time.Second*time.Duration(result.ExpiresIn) - time.Second*5)
	print("REFRESHED moveyou")
}
