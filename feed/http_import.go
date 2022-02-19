package feed

import (
	"log"
	"net/http"
	"time"
)

func (feed Feed) DownloadData(url string) *http.Response {
	client := &http.Client{
		Timeout: 10 * time.Second,
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

func (feed Feed) addAuth(r *http.Request) *http.Request {
	switch feed.AuthenticationType {
	case "oauth2":
		token := feed.OAuth2Credentials.GetAccessToken()
		r.Header.Add("authorization", "Bearer "+token)
	case "token":
		if feed.ApiKeyName != "" {
			r.Header.Add(feed.ApiKeyName, feed.ApiKey)
		}
	}

	return r
}

func (feed Feed) addAdditionalRequestHeaders(r *http.Request) *http.Request {
	for key, value := range feed.RequestHeaders {
		r.Header.Add(key, value)
	}
	return r
}
