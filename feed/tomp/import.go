package tomp

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type AvailableAssets []struct {
	TypeID          string        `json:"typeId"`
	Name            string        `json:"name"`
	AssetClass      string        `json:"assetClass"`
	AmountAvailable int           `json:"amountAvailable"`
	Assets          []Asset       `json:"assets"`
	Fuel            string        `json:"fuel"`
	EnergyLabel     string        `json:"energyLabel"`
	TravelAbroad    bool          `json:"travelAbroad"`
	Image           string        `json:"image"`
	Propulsion      string        `json:"propulsion"`
	Smoking         bool          `json:"smoking"`
	Meta            []interface{} `json:"meta"`
}

type Asset struct {
	TypeID       string        `json:"typeId,omitempty"`
	Name         string        `json:"name,omitempty"`
	AssetClass   string        `json:"assetClass,omitempty"`
	Assets       []interface{} `json:"assets,omitempty"`
	EnergyLabel  string        `json:"energyLabel,omitempty"`
	TravelAbroad bool          `json:"travelAbroad,omitempty"`
	Smoking      bool          `json:"smoking,omitempty"`
	Meta         []interface{} `json:"meta,omitempty"`
	AssetID      string        `json:"assetId,omitempty"`
	Place        struct {
		StopReference []interface{} `json:"stopReference"`
		StationID     string        `json:"stationId"`
		Coordinates   struct {
			Lng float64 `json:"lng"`
			Lat float64 `json:"lat"`
		} `json:"coordinates"`
		PhysicalAddress struct {
			StreetAddress string `json:"streetAddress"`
			AreaReference string `json:"areaReference"`
			PostalCode    string `json:"postalCode"`
			Country       string `json:"country"`
		} `json:"physicalAddress"`
	} `json:"place,omitempty"`
	Image string `json:"image,omitempty"`
}

func ImportFeed(feed *feed.Feed) []feed.Bike {
	feed.NumberOfPulls = feed.NumberOfPulls + 1
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

	res, err := client.Do(req)
	if err != nil {
		log.Print(err)
		return nil
	}
	if res.StatusCode != http.StatusOK {
		log.Printf("[%s] Loading data from %s not possible. Status code: %d", feed.OperatorID, feed.Url, res.StatusCode)
		return nil
	}

	decoder := json.NewDecoder(res.Body)
	var bikeFeed AvailableAssets
	decoder.Decode(&bikeFeed)
	return convertTompToFreeBike(bikeFeed)
}

func convertTompToFreeBike(tompFeed AvailableAssets) []feed.Bike {
	bikes := []feed.Bike{}
	for _, availableAssets := range tompFeed {
		newBikes := converTompAssetsToBikes(availableAssets.Assets)
		bikes = append(bikes, newBikes...)
	}
	return bikes
}

func converTompAssetsToBikes(assetsTomp []Asset) []feed.Bike {
	bikes := []feed.Bike{}
	for _, tompBike := range assetsTomp {
		bike := feed.Bike{
			BikeID:     tompBike.AssetID,
			Lat:        tompBike.Place.Coordinates.Lat,
			Lon:        tompBike.Place.Coordinates.Lng,
			IsReserved: false,
			IsDisabled: false,
		}
		bikes = append(bikes, bike)
	}
	return bikes
}
