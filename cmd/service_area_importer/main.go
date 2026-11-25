package main

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/feed_status"
	"deelfietsdashboard-importer/feed/gbfs"
	"deelfietsdashboard-importer/process"
	"encoding/json"
	"log"
)

func main() {
	log.Print("Start service_area_importer")
	dataProcessor := process.InitDataProcessor()
	feeds := process.LoadGeofencingFeeds(dataProcessor)

	data := []gbfs.GBFSGeofencing{}
	for _, dataFeed := range feeds {
		geofences, supported := downloadFeed(dataFeed)
		if !supported {
			continue
		}
		// A feed without zones counts as failing: a failed download, an
		// unreadable response and an empty feed all end up here.
		if countZones(geofences) > 0 {
			feed_status.RecordImportSuccess(dataFeed.ID, dataProcessor.DB)
		} else {
			log.Printf("[%s_%d] no geofencing zones in feed", dataFeed.OperatorID, dataFeed.ID)
			feed_status.RecordImportFailure(dataFeed.ID, dataProcessor.DB)
		}
		data = append(data, geofences...)
	}
	dataProcessor.ProcessGeofences(data)
}

// downloadFeed downloads the geofencing zones of one feed. A manifest feed
// refers to several GBFS feeds, so the result can hold more than one item.
// The second result is false when the feed type is not supported.
func downloadFeed(dataFeed feed.Feed) ([]gbfs.GBFSGeofencing, bool) {
	switch dataFeed.Type {
	case "manifest_gbfs":
		res := []gbfs.GBFSGeofencing{}
		for _, loadedFeed := range gbfs.LoadFeedsFromManifest(dataFeed) {
			geofences, _ := downloadFeed(loadedFeed)
			res = append(res, geofences...)
		}
		return res, true
	case "full_gbfs":
		return []gbfs.GBFSGeofencing{gbfs.ImportFullGeofenceV3(dataFeed)}, true
	case "gbfs":
		log.Print("Start importing gbfs")
		return []gbfs.GBFSGeofencing{gbfs.ImportGeofence(dataFeed, dataFeed.Url)}, true
	default:
		log.Printf("NOT SUPPORTED: %s", dataFeed.Type)
		return nil, false
	}
}

// countZones returns the number of geofencing zones in the downloaded data.
func countZones(geofences []gbfs.GBFSGeofencing) int {
	total := 0
	for _, geofence := range geofences {
		var collection struct {
			Features []json.RawMessage `json:"features"`
		}
		if err := json.Unmarshal(geofence.Data.GeofencingZones, &collection); err != nil {
			continue
		}
		total += len(collection.Features)
	}
	return total
}
