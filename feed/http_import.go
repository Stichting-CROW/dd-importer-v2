package feed

import (
	"log"
	"net/http"
	"time"
)

func (feed Feed) DownloadData() *http.Response {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", feed.Url, nil)
	if err != nil {
		log.Print(err)
		return nil
	}
	req = feed.addAuth(req)

	res, err := client.Do(req)
	if err != nil {
		log.Print(err)
		return nil
	}
	log.Print(res.Status)
	if res.StatusCode != http.StatusOK {
		log.Printf("[%s] Loading data from %s not possible. Status code: %d", feed.OperatorID, feed.Url, res.StatusCode)
		return nil
	}
	return res

}

func (feed Feed) addAuth(r *http.Request) *http.Request {
	switch feed.AuthenticationType {
	case "oauth2":
		token := feed.OAuth2Credentials.GetAccessToken()
		log.Print(token)
		r.Header.Add("authorization", "Bearer "+token)
	case "token":
		if feed.ApiKeyName != "" {
			r.Header.Add(feed.ApiKeyName, feed.ApiKey)
		}
	}

	return r
}
