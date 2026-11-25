package feed

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"
)

func ecPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
}

func TestParseAuthentication(t *testing.T) {
	dottConfig := func(privateKey string, audience []string) string {
		encoded, _ := json.Marshal(map[string]any{
			"authentication_type": "oauth2-dott",
			"OAuth2Credentials": map[string]any{
				"audience": audience, "keyId": "kid", "organizationId": "org", "privateKey": privateKey,
			},
		})
		return string(encoded)
	}

	cases := []struct {
		name     string
		config   string
		wantType string
		wantErr  bool
	}{
		{"no authentication", ``, "", false},
		{"null", `null`, "", false},
		{"without authentication_type", `{"foo":"bar"}`, "", false},
		{"unknown type is ignored", `{"authentication_type":"magic"}`, "", false},
		{"token", `{"authentication_type":"token","ApiKeyName":"X-Key","ApiKey":"s"}`, "token", false},
		{"token without key", `{"authentication_type":"token","ApiKeyName":"X-Key"}`, "", true},
		{"oauth2", `{"authentication_type":"oauth2","TokenURL":"https://t","OAuth2Credentials":
			{"client_id":"a","client_secret":"b","grant_type":"c","scope":"d"}}`, "oauth2", false},
		{"oauth2 without scope", `{"authentication_type":"oauth2","TokenURL":"https://t","OAuth2Credentials":
			{"client_id":"a","client_secret":"b","grant_type":"c"}}`, "", true},
		{"basic auth without scope", `{"authentication_type":"oauth2-basic-auth","TokenURL":"https://t",
			"OAuth2Credentials":{"client_id":"a","client_secret":"b"}}`, "oauth2-basic-auth", false},
		{"basic auth with numeric scope", `{"authentication_type":"oauth2-bolt","TokenURL":"https://t",
			"OAuth2Credentials":{"client_id":"a","client_secret":"b","scope":1}}`, "", true},
		{"moveyou without token url", `{"authentication_type":"oauth2-moveyou",
			"OAuth2Credentials":{"client_id":"a","client_secret":"b","grant_type":"c"}}`, "", true},
		{"dott", dottConfig(ecPrivateKeyPEM(t), []string{"https://dott"}), "oauth2-dott", false},
		{"dott with invalid key", dottConfig("not a key", []string{"https://dott"}), "", true},
		{"dott without audience", dottConfig(ecPrivateKeyPEM(t), []string{}), "", true},
		{"not json", `{"authentication_type":`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := Feed{}
			err := ParseAuthentication(&f, []byte(c.config))
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if f.AuthenticationType != c.wantType {
				t.Fatalf("AuthenticationType = %q, want %q", f.AuthenticationType, c.wantType)
			}
		})
	}
}
