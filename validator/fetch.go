package validator

import (
	"bytes"
	"context"
	"deelfietsdashboard-importer/feed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	maxBodyBytes      = 64 << 20
	maxRequestTimeout = 30 * time.Second
)

// safeQueryParams are query parameters whose values are shown unmasked.
var safeQueryParams = map[string]bool{
	"end_time": true, "size": true, "page[offset]": true, "page[limit]": true,
}

type run struct {
	ctx    context.Context
	feed   feed.Feed
	result *Result
}

// download fetches url with the feed's authentication. It returns the body
// only for a 200 response; otherwise the reason is stored on the file.
func (r *run) download(file *FileResult, rawURL string) ([]byte, bool) {
	body, status, ok := r.downloadStatus(file, rawURL)
	if ok && status != http.StatusOK {
		file.Messages = append(file.Messages, describeStatus(status))
		return nil, false
	}
	if ok {
		file.Reachable = true
	}
	return body, ok
}

// downloadStatus is like download but leaves non-200 responses to the caller.
func (r *run) downloadStatus(file *FileResult, rawURL string) ([]byte, int, bool) {
	timeout := maxRequestTimeout
	if deadline, ok := r.ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= time.Second {
			file.Messages = append(file.Messages, "Niet opgehaald: de tijdslimiet van de validatie is bereikt.")
			return nil, 0, false
		}
		if remaining < timeout {
			timeout = remaining
		}
	}

	res, err := r.feed.Fetch(rawURL, timeout)
	if err != nil {
		file.Messages = append(file.Messages, describeFetchError(err))
		return nil, 0, false
	}
	defer res.Body.Close()
	file.HTTPStatus = res.StatusCode

	body, err := io.ReadAll(io.LimitReader(res.Body, maxBodyBytes+1))
	if err != nil {
		file.Messages = append(file.Messages, "Het antwoord kon niet volledig gelezen worden.")
		return nil, res.StatusCode, false
	}
	if len(body) > maxBodyBytes {
		file.Messages = append(file.Messages, fmt.Sprintf("Het antwoord is groter dan %d MB en is niet gecontroleerd.", maxBodyBytes>>20))
		return nil, res.StatusCode, false
	}
	return body, res.StatusCode, true
}

// downloadJSON downloads and parses a JSON file. A body that is not JSON
// marks the file as invalid.
func (r *run) downloadJSON(file *FileResult, rawURL string) (any, bool) {
	body, ok := r.download(file, rawURL)
	if !ok {
		return nil, false
	}
	return parseJSON(file, body)
}

func parseJSON(file *FileResult, body []byte) (any, bool) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		file.Messages = append(file.Messages, "Het antwoord is geen geldige JSON.")
		file.Valid = false
		return nil, false
	}
	return doc, true
}

func describeStatus(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Sprintf("HTTP %d: toegang geweigerd, controleer de credentials.", status)
	case http.StatusNotFound:
		return "HTTP 404: de URL bestaat niet."
	case http.StatusTooManyRequests:
		return "HTTP 429: te veel verzoeken, probeer het later opnieuw."
	default:
		return fmt.Sprintf("HTTP %d: de aanbieder gaf geen geldig antwoord.", status)
	}
}

// describeFetchError explains a failed request without repeating the error
// text, because that contains the full URL.
func describeFetchError(err error) string {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Timeout() {
		return "Geen antwoord binnen de tijdslimiet."
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "no such host"):
		return "De server bestaat niet (DNS)."
	case strings.Contains(message, "connection refused"):
		return "De server weigert de verbinding."
	case strings.Contains(message, "certificate"), strings.Contains(message, "tls"):
		return "Er is een probleem met het TLS-certificaat van de server."
	case strings.Contains(message, "unsupported protocol scheme"):
		return "De URL moet met http:// of https:// beginnen."
	default:
		return "De verbinding met de server is mislukt."
	}
}

// MaskURL hides user info and query values that may hold credentials.
func MaskURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "(ongeldige URL)"
	}
	hadUser := u.User != nil
	u.User = nil
	if u.RawQuery != "" {
		u.RawQuery = maskQuery(u.RawQuery)
	}
	masked := u.String()
	if hadUser {
		masked = strings.Replace(masked, "://", "://•••@", 1)
	}
	return masked
}

func maskQuery(rawQuery string) string {
	parts := strings.Split(rawQuery, "&")
	for i, part := range parts {
		key, _, hasValue := strings.Cut(part, "=")
		decodedKey, err := url.QueryUnescape(key)
		if err != nil {
			decodedKey = key
		}
		if hasValue && !safeQueryParams[decodedKey] {
			parts[i] = key + "=•••"
		}
	}
	return strings.Join(parts, "&")
}
