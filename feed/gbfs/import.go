package gbfs

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"io"
	"log"
)

// FreeBikeStatusV2 is the struct that represents the FreeBikeStatusV2	 gbfs structure.
type FreeBikeStatusV2 struct {
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

func GetBikesFeedV2(data []byte) []feed.Bike {
	var bikeFeed FreeBikeStatusV2
	err := json.Unmarshal(data, &bikeFeed)
	if err != nil {
		return nil
	}

	return bikeFeed.Data.Bikes
}

func getData(dataFeed *feed.Feed, url string) []feed.Bike {
	res := dataFeed.DownloadData(url)
	if res == nil {
		return nil
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Something went wrong while reading the response body: %s", url)
		return nil
	}

	GBFSVersion := 0
	if dataFeed.OperatorID == "cykl" {
		GBFSVersion = 1
	} else if dataFeed.OperatorID == "deelfietsnederland" {
		GBFSVersion = 2
	} else {
		GBFSVersion, err = GetMajorVersionFromResponse(data)
		if err != nil {
			return nil
		}
	}

	var bikes []feed.Bike
	switch GBFSVersion {
	case 1:
		log.Printf("Importing feed with GBFS version 1: %s", url)
		bikes = GetBikesFeedV1(data)
	case 2:
		log.Printf("Importing feed with GBFS version 2: %s", url)
		bikes = GetBikesFeedV2(data)
	default:
		log.Printf("Major version %d not supported: %s", GBFSVersion, url)
		return nil
	}

	for i := range bikes {
		bikes[i].SystemID = dataFeed.OperatorID
	}
	return bikes
}
