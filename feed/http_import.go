package feed

import (
	"log"
	"net/http"
	"time"
)

func (feed *Feed) DownloadData(url string) *http.Response {
	return feed.DownloadDataAllowTimeout(url, time.Second*15)

}

func (feed *Feed) DownloadDataAllowTimeout(url string, seconds time.Duration) *http.Response {
	client := &http.Client{
		Timeout: seconds * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Print(err)
		return nil
	}
	req = feed.addAuth(req)
	req = feed.addAdditionalRequestHeaders(req)

	res, err := client.Do(req)
	if err != nil {
		log.Print(err)
		return nil
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("[%s] Loading data from %s not possible. Status code: %d", feed.OperatorID, url, res.StatusCode)
		return nil
	}
	return res

}

func (feed *Feed) addAuth(r *http.Request) *http.Request {
	switch feed.AuthenticationType {
	case "oauth2":
		token := feed.OAuth2Credentials.GetAccessToken()
		r.Header.Add("authorization", "Bearer "+token)
	case "token":
		if feed.ApiKeyName != "" {
			r.Header.Add(feed.ApiKeyName, feed.ApiKey)
		}
	case "oauth2-gosharing":
		token := feed.OAuth2CredentialsGosharing.GetAccessToken()
		r.Header.Add("authorization", "Bearer "+token)
	case "oauth2-bolt":
		token := feed.OAuth2CredentialsBolt.GetAccessToken()
		r.Header.Add("authorization", "Bearer "+token)
	case "oauth2-moveyou":
		token := feed.OAuth2CredentialsMoveyou.GetAccessToken()
		r.Header.Add("authorization", "Bearer "+token)
	case "oauth2-dott":
		token := feed.OAuth2CredentialsDott.GetAccessToken()
		r.Header.Add("authorization", "Bearer "+token)
	}

	return r
}

func (feed Feed) addAdditionalRequestHeaders(r *http.Request) *http.Request {
	for key, value := range feed.RequestHeaders {
		r.Header.Add(key, value)
	}
	return r
}
