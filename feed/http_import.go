package feed

import (
	"log"
	"net/http"
	"time"
)

const (
	// maxRateLimitRetries is how often a 429 is retried before giving up.
	maxRateLimitRetries = 7
	// maxRateLimitBackoff caps the exponential backoff between retries.
	maxRateLimitBackoff = time.Minute
)

func (feed *Feed) DownloadData(url string) *http.Response {
	return feed.DownloadDataAllowTimeout(url, time.Second*15)
}

func (feed *Feed) DownloadDataAllowTimeout(url string, seconds time.Duration) *http.Response {
	res, status := feed.DownloadDataAllowTimeoutStatus(url, seconds)
	if status != http.StatusOK {
		if res != nil {
			res.Body.Close()
		}
		return nil
	}
	return res
}

// DownloadDataAllowTimeoutStatus performs a single request and, unlike
// DownloadDataAllowTimeout, also returns the HTTP status code (-1 when the
// request could not be sent). A non-OK response is still returned so callers
// can handle specific statuses themselves. Non-OK responses are always logged.
func (feed *Feed) DownloadDataAllowTimeoutStatus(url string, seconds time.Duration) (*http.Response, int) {
	res, status := feed.doRequest(url, seconds)
	if status != http.StatusOK {
		log.Printf("[%s_%d] Loading data from %s not possible. Status code: %d", feed.OperatorID, feed.ID, url, status)
	}
	return res, status
}

// DownloadDataTripsWithRetry is like DownloadDataAllowTimeoutStatus but retries
// a 429 (rate limiting) with exponential backoff capped at one minute. Only
// used for trip imports; vehicle imports must not retry.
func (feed *Feed) DownloadDataTripsWithRetry(url string, seconds time.Duration) (*http.Response, int) {
	backoff := time.Second
	for attempt := 0; ; attempt++ {
		res, status := feed.doRequest(url, seconds)

		if status == http.StatusTooManyRequests && attempt < maxRateLimitRetries {
			if res != nil {
				res.Body.Close()
			}
			log.Printf("[%s_%d] rate limited (429) on %s, retrying in %v", feed.OperatorID, feed.ID, url, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxRateLimitBackoff {
				backoff = maxRateLimitBackoff
			}
			continue
		}

		if status != http.StatusOK {
			log.Printf("[%s_%d] Loading data from %s not possible. Status code: %d", feed.OperatorID, feed.ID, url, status)
		}
		return res, status
	}
}

func (feed *Feed) doRequest(url string, seconds time.Duration) (*http.Response, int) {
	client := &http.Client{
		Timeout: seconds * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Print(err)
		return nil, -1
	}
	req = feed.addAuth(req)
	req = feed.addAdditionalRequestHeaders(req)

	res, err := client.Do(req)
	if err != nil {
		log.Print(err)
		return nil, -1
	}
	return res, res.StatusCode
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
	case "oauth2-bolt":
		token := feed.OAuth2CredentialsBasicAuth.GetAccessToken()
		r.Header.Add("authorization", "Bearer "+token)
	case "oauth2-basic-auth":
		token := feed.OAuth2CredentialsBasicAuth.GetAccessToken()
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
