package feed

import (
	"deelfietsdashboard-importer/feed/auth"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt"
)

// AuthenticationTypes lists every value of authentication.authentication_type
// that addAuth knows how to apply.
var AuthenticationTypes = []string{
	"token", "oauth2", "oauth2-bolt", "oauth2-basic-auth", "oauth2-moveyou", "oauth2-dott",
}

// ParseAuthentication reads the feeds.authentication JSON into the feed.
//
// Every field the auth code reads is checked here, because the token refresh
// functions use unchecked type assertions (and log.Fatalf for Dott) that would
// otherwise crash the whole process on a malformed row. An empty value or one
// without authentication_type means the feed needs no authentication. An
// unknown authentication_type is ignored, as before.
func ParseAuthentication(f *Feed, data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("authentication is not a JSON object: %w", err)
	}
	if _, ok := config["authentication_type"]; !ok {
		return nil
	}
	authenticationType, ok := config["authentication_type"].(string)
	if !ok {
		return errors.New("authentication_type must be a string")
	}

	switch authenticationType {
	case "token":
		name, err := requiredString(config, "ApiKeyName")
		if err != nil {
			return err
		}
		key, err := requiredString(config, "ApiKey")
		if err != nil {
			return err
		}
		f.ApiKeyName, f.ApiKey = name, key
	case "oauth2":
		body, tokenURL, err := oauthConfig(config, "client_id", "client_secret", "grant_type", "scope")
		if err != nil {
			return err
		}
		f.OAuth2Credentials.OauthTokenBody, f.OAuth2Credentials.TokenURL = body, tokenURL
	case "oauth2-bolt", "oauth2-basic-auth":
		body, tokenURL, err := oauthConfig(config, "client_id", "client_secret")
		if err != nil {
			return err
		}
		if scope, ok := body["scope"]; ok && scope != nil {
			if _, ok := scope.(string); !ok {
				return errors.New("OAuth2Credentials.scope must be a string")
			}
		}
		f.OAuth2CredentialsBasicAuth.OauthTokenBody, f.OAuth2CredentialsBasicAuth.TokenURL = body, tokenURL
	case "oauth2-moveyou":
		body, tokenURL, err := oauthConfig(config, "client_id", "client_secret", "grant_type")
		if err != nil {
			return err
		}
		f.OAuth2CredentialsMoveyou.OauthTokenBody, f.OAuth2CredentialsMoveyou.TokenURL = body, tokenURL
	case "oauth2-dott":
		dottConfig, err := dottAuthConfig(config)
		if err != nil {
			return err
		}
		f.OAuth2CredentialsDott.OauthTokenBody = dottConfig
	default:
		return nil
	}

	f.AuthenticationType = authenticationType
	return nil
}

func requiredString(config map[string]any, key string) (string, error) {
	value, ok := config[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return value, nil
}

func oauthConfig(config map[string]any, requiredKeys ...string) (map[string]any, string, error) {
	tokenURL, err := requiredString(config, "TokenURL")
	if err != nil {
		return nil, "", err
	}
	body, ok := config["OAuth2Credentials"].(map[string]any)
	if !ok {
		return nil, "", errors.New("OAuth2Credentials must be an object")
	}
	for _, key := range requiredKeys {
		if _, ok := body[key].(string); !ok {
			return nil, "", fmt.Errorf("OAuth2Credentials.%s must be a string", key)
		}
	}
	return body, tokenURL, nil
}

func dottAuthConfig(config map[string]any) (auth.DottAuthConfig, error) {
	var dottConfig auth.DottAuthConfig
	raw, err := json.Marshal(config["OAuth2Credentials"])
	if err != nil {
		return dottConfig, err
	}
	if err := json.Unmarshal(raw, &dottConfig); err != nil {
		return dottConfig, errors.New("OAuth2Credentials is not a valid Dott configuration")
	}
	if len(dottConfig.Audience) == 0 || dottConfig.Audience[0] == "" {
		return dottConfig, errors.New("OAuth2Credentials.audience must contain at least one value")
	}
	if dottConfig.KeyID == "" || dottConfig.OrganizationID == "" {
		return dottConfig, errors.New("OAuth2Credentials.keyId and organizationId are required")
	}
	if _, err := jwt.ParseECPrivateKeyFromPEM([]byte(dottConfig.PrivateKey)); err != nil {
		return dottConfig, errors.New("OAuth2Credentials.privateKey is not a valid EC private key")
	}
	return dottConfig, nil
}
