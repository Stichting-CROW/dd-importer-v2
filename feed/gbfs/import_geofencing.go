package gbfs

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
)

type Rule struct {
	RideStartAllowed   *bool    `json:"ride_start_allowed"`
	RideEndAllowed     *bool    `json:"ride_end_allowed"`
	RideThroughAllowed *bool    `json:"ride_through_allowed"`
	MaximumSpeed       *float64 `json:"maximum_speed_kph"`
}

type GBFSGeofencing struct {
	OperatorID string
	Data       struct {
		GeofencingZones json.RawMessage `json:"geofencing_zones"`
		GlobalRules     []Rule          `json:"global_rules"`
	} `json:"data"`
	LastUpdated int    `json:"last_updated"`
	TTL         int    `json:"ttl"`
	Version     string `json:"version"`
}

func ImportGeofence(feed feed.Feed, url string) GBFSGeofencing {
	res := feed.DownloadData(url)

	if res == nil {
		return GBFSGeofencing{}
	}
	var geofencingGBFS GBFSGeofencing
	json.NewDecoder(res.Body).Decode(&geofencingGBFS)
	geofencingGBFS.OperatorID = feed.OperatorID
	return geofencingGBFS
}
