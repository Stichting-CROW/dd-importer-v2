package gbfs

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
)

type GBFSGeofencing struct {
	OperatorID string
	Data       struct {
		GeofencingZones json.RawMessage `json:"geofencing_zones"`
		GlobalRules     []struct {
			RideEndAllowed     bool `json:"ride_end_allowed"`
			RideStartAllowed   bool `json:"ride_start_allowed"`
			RideThroughAllowed bool `json:"ride_through_allowed"`
		} `json:"global_rules"`
	} `json:"data"`
	LastUpdated int    `json:"last_updated"`
	TTL         int    `json:"ttl"`
	Version     string `json:"version"`
}

func ImportGeofence(feed feed.Feed, url string) GBFSGeofencing {
	res := feed.DownloadData(url)

	var geofencingGBFS GBFSGeofencing
	json.NewDecoder(res.Body).Decode(&geofencingGBFS)
	geofencingGBFS.OperatorID = feed.OperatorID
	return geofencingGBFS
}
