package gbfs

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
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
	return getData(feed, feed.Url)
}

func getData(feed *feed.Feed, url string) []feed.Bike {
	res := feed.DownloadData(url)
	if res == nil {
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
