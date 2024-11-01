package auth

import (
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
)

type DottAuthConfig struct {
	Audience       []string `json:"audience"`
	KeyID          string   `json:"keyId"`
	OrganizationID string   `json:"organizationId"`
	PrivateKey     string   `json:"privateKey"`
}

type OauthCredentialsDott struct {
	ExpireTime     time.Time
	TokenURL       string
	AccessToken    string
	OauthTokenBody DottAuthConfig
}

func (o *OauthCredentialsDott) GetAccessToken() string {
	if time.Now().After(o.ExpireTime) {
		log.Print("Get new accesstoken Dott.")
		o.refreshToken()
	}
	return o.AccessToken
}

func (o *OauthCredentialsDott) refreshToken() {
	// Parse the private key
	config := o.OauthTokenBody
	privateKey, err := jwt.ParseECPrivateKeyFromPEM([]byte(config.PrivateKey))
	if err != nil {
		log.Fatalf("Error parsing EC private key: %v", err)
	}

	// Create an empty payload
	payload := jwt.MapClaims{}

	// Set up the JWT header and options
	// Algorithm must be ES256, with provided `kid`
	token := jwt.NewWithClaims(jwt.SigningMethodES256, payload)
	token.Header["kid"] = config.KeyID

	exp := time.Now().Add(3600 * time.Second)
	// Set options in the payload (claims)
	payload["exp"] = exp.Unix() // expiration in 60 seconds
	payload["iss"] = fmt.Sprintf("%s@external.organization.ridedott.com", config.OrganizationID)
	payload["aud"] = config.Audience[0]
	payload["jti"] = uuid.New().String() // random UUID for each request

	// Sign the token
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		log.Print("Something went wrong with signin token string DOTT.")
	}
	o.AccessToken = tokenString
	o.ExpireTime = exp.Add(-5 * time.Second)
}
