package tomp

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
)

type AssetType []struct {
	ID            string  `json:"id"`
	AssetClass    string  `json:"assetClass"`
	AssetSubClass string  `json:"assetSubClass"`
	Assets        []Asset `json:"assets"`
}

type Asset struct {
	ID                   string `json:"id"`
	IsReserved           bool   `json:"isReserved"`
	IsDisabled           bool   `json:"isDisabled"`
	OverriddenProperties struct {
		Location struct {
			Coordinates struct {
				Lng float64 `json:"lng"`
				Lat float64 `json:"lat"`
			} `json:"coordinates"`
		} `json:"location"`
		Fuel string `json:"fuel"`
	} `json:"overriddenProperties"`
}

func ImportFeed(feed *feed.Feed) []feed.Bike {
	feed.NumberOfPulls = feed.NumberOfPulls + 1
	return getData(feed)
}

func getData(feed *feed.Feed) []feed.Bike {
	res := feed.DownloadData(feed.Url)
	if res == nil {
		return nil
	}

	decoder := json.NewDecoder(res.Body)
	var bikeFeed AssetType
	decoder.Decode(&bikeFeed)
	return convertTompToFreeBike(bikeFeed, feed.OperatorID)
}

func convertTompToFreeBike(tompFeed AssetType, systemID string) []feed.Bike {
	bikes := []feed.Bike{}
	for _, availableAssets := range tompFeed {
		// Temporary read only tomp bicycles.
		if availableAssets.AssetClass == "BICYCLE" {
			newBikes := convertTompAssetsToBikes(availableAssets.Assets, systemID)
			bikes = append(bikes, newBikes...)
		}
	}
	return bikes
}

func convertTompAssetsToBikes(assetsTomp []Asset, systemID string) []feed.Bike {
	bikes := []feed.Bike{}
	for _, tompBike := range assetsTomp {
		bike := feed.Bike{
			BikeID:     tompBike.ID,
			Lat:        tompBike.OverriddenProperties.Location.Coordinates.Lat,
			Lon:        tompBike.OverriddenProperties.Location.Coordinates.Lng,
			IsReserved: tompBike.IsReserved,
			IsDisabled: tompBike.IsDisabled,
			SystemID:   systemID,
		}
		bikes = append(bikes, bike)
	}
	return bikes
}
