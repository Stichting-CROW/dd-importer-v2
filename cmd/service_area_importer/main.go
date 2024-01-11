package main

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/gbfs"
	"deelfietsdashboard-importer/process"
	"log"
)

func main() {
	dataProcessor := process.InitDataProcessor()
	feeds := process.LoadGeofencingFeeds(dataProcessor)
	data := downloadFeeds(feeds)
	dataProcessor.ProcessGeofences(data)
}

func downloadFeeds(feeds []feed.Feed) []gbfs.GBFSGeofencing {
	res := []gbfs.GBFSGeofencing{}
	for _, dataFeed := range feeds {
		switch dataFeed.Type {
		case "manifest_gbfs":
			loadedFeeds := gbfs.LoadFeedsFromManifest(dataFeed)
			res = downloadFeeds(loadedFeeds)
		case "full_gbfs":
			res = append(res, gbfs.ImportFullGeofenceV3(dataFeed))
		default:
			log.Printf("NOT SUPPORTED: %s", dataFeed.Type)
		}
	}
	return res
}
