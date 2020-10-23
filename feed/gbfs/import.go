package gbfs

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// FreeBikeStatus is the struct that represents the FreeBikeStatus gbfs structure.
type FreeBikeStatus struct {
	LastUpdated int `json:"last_updated"`
	TTL         int `json:"ttl"`
	Data        struct {
		Bikes []feed.Bike `json:"bikes"`
	} `json:"data"`
}

// ImportFeed is a function is called to import data from a feed.
func ImportFeed(feed *feed.Feed) []feed.Bike {
	return getData(feed)
}

func getData(feed *feed.Feed) []feed.Bike {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", feed.Url, nil)
	if err != nil {
		log.Print(err)
		return nil
	}
	if feed.ApiKeyName != "" {
		req.Header.Add(feed.ApiKeyName, feed.ApiKey)
	}

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

	decoder := json.NewDecoder(res.Body)
	var bikeFeed FreeBikeStatus
	decoder.Decode(&bikeFeed)

	// Set SystemID
	bikes := bikeFeed.Data.Bikes
	for i := range bikes {
		bikes[i].SystemID = feed.OperatorID
	}
	return bikes
}
