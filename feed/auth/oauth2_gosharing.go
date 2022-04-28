package auth

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type OauthCredentialsGosharing struct {
	ExpireTime     time.Time
	TokenURL       string
	AccessToken    string
	OauthTokenBody map[string]interface{}
}

func (o *OauthCredentialsGosharing) GetAccessToken() string {
	log.Print("Check accessToken")
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

type OAuthResultGosharing struct {
	Data struct {
		Mfastatus              string    `json:"mfaStatus"`
		Accesstokenexpiredate  time.Time `json:"accessTokenExpireDate"`
		Refreshtokenexpiredate time.Time `json:"refreshTokenExpireDate"`
		Mfarequired            bool      `json:"mfaRequired"`
		Accesstoken            string    `json:"accessToken"`
		UUID                   string    `json:"uuid"`
		Refreshtoken           string    `json:"refreshToken"`
	} `json:"data"`
}

func (o *OauthCredentialsGosharing) refreshToken() {
	log.Print("Refresh token gosharing")
	jsonValue, _ := json.Marshal(o.OauthTokenBody)

	resp, err := http.Post(o.TokenURL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Print(err)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	var result OAuthResultGosharing
	json.Unmarshal(body, &result)
	o.AccessToken = result.Data.Accesstoken
	expireTime := result.Data.Accesstokenexpiredate.Add(time.Duration(time.Second * -5))
	o.ExpireTime = expireTime
	log.Print(o)
}
