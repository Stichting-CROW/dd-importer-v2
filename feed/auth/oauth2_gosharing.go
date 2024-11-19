package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type OauthCredentialsGosharing struct {
	ExpireTime     time.Time
	TokenURL       string
	AccessToken    string
	OauthTokenBody map[string]interface{}
}

func (o *OauthCredentialsGosharing) GetAccessToken() string {
	if time.Now().After(o.ExpireTime) {
		o.refreshToken()
	}
	return o.AccessToken
}

func (o *OauthCredentialsGosharing) refreshToken() {
	log.Print("Refresh token gosharing")

	clientID := o.OauthTokenBody["client_id"].(string)
	clientSecret := o.OauthTokenBody["client_secret"].(string)

	// Encode clientID and clientSecret in base64
	auth := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	params := url.Values{}
	params.Set("grant_type", o.OauthTokenBody["grant_type"].(string))
	request_params := bytes.NewBufferString(params.Encode())

	req, _ := http.NewRequest("POST", o.TokenURL, request_params)
	req.Header.Add("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
	}
	defer resp.Body.Close()
	log.Printf("Statuscode %d", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	var result OAuthResult
	json.Unmarshal(body, &result)
	o.AccessToken = result.AccessToken
	o.ExpireTime = time.Now().Add(time.Second*time.Duration(result.ExpiresIn) - time.Second*5)
}
