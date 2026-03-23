package gbfs

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
)

type GBFSBikeV1 struct {
	BikeID     string  `json:"bike_id"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	IsReserved int     `json:"is_reserved"`
	IsDisabled int     `json:"is_disabled"`
}

type FreeBikeStatusV1 struct {
	LastUpdated int `json:"last_updated"`
	TTL         int `json:"ttl"`
	Data        struct {
		Bikes []GBFSBikeV1 `json:"bikes"`
	} `json:"data"`
}

func convertV1ToFeedBike(bike GBFSBikeV1) feed.Bike {
	return feed.Bike{
		BikeID:     bike.BikeID,
		Lat:        bike.Lat,
		Lon:        bike.Lon,
		IsReserved: bike.IsReserved == 1,
		IsDisabled: bike.IsDisabled == 1,
	}
}

func GetBikesFeedV1(data []byte) []feed.Bike {
	var bikeFeed FreeBikeStatusV1
	err := json.Unmarshal(data, &bikeFeed)
	if err != nil {
		return nil
	}

	bikes := make([]feed.Bike, len(bikeFeed.Data.Bikes))
	for i, bike := range bikeFeed.Data.Bikes {
		bikes[i] = convertV1ToFeedBike(bike)
	}
	return bikes
}
