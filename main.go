package main

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/gbfs"
	"deelfietsdashboard-importer/feed/tomp"
	"deelfietsdashboard-importer/process"
	"log"
	"sync"
	"time"
)

func main() {
	felyx := feed.Feed{
		OperatorID:     "felyx",
		Url:            "https://data.felyx.com/gbfs/free_bike_status.json",
		ApiKeyName:     "x-api-key",
		ApiKey:         "dfisVeyzZfhc289Dxn9Ap7AeZwTFt3fjGpf28C9st9VoBiS6vAwvtdp8GHZQezn3b5cHKJ2hW39z7eCHsh7pf5atXfaQLfegpV7fWC9pvW42C5jLTJa3CiNdBrGmBeYy",
		NumberOfPulls:  0,
		Type:           "gbfs",
		ImportStrategy: "clean",
	}
	hely := feed.Feed{
		OperatorID:     "hely",
		Url:            "https://tomp.hely.com/operator/available-assets",
		NumberOfPulls:  0,
		Type:           "tomp",
		ImportStrategy: "clean",
	}
	keobike := feed.Feed{
		OperatorID:     "keobike",
		Url:            "https://api.mobilock.nl/gbfs/v2/free-bike-status/keobike",
		NumberOfPulls:  0,
		Type:           "gbfs",
		ImportStrategy: "clean",
	}
	feeds := []feed.Feed{}

	feeds = append(feeds, hely)
	feeds = append(feeds, felyx)
	feeds = append(feeds, keobike)
	importLoop(feeds)
}

func importLoop(feeds []feed.Feed) {
	var waitGroup sync.WaitGroup

	for {
		startImport := time.Now()
		for index, _ := range feeds {
			waitGroup.Add(1)
			go importFeed(&feeds[index], &waitGroup)
		}
		waitGroup.Wait()
		importDuration := time.Now().Sub(startImport)
		log.Printf("Import took %v", importDuration)
		if importDuration.Seconds() <= 30 {
			time.Sleep(time.Second*30 - importDuration)
		}
	}
}

func importFeed(operator_feed *feed.Feed, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	var newBikes []feed.Bike
	switch operator_feed.Type {
	case "gbfs":
		newBikes = gbfs.ImportFeed(operator_feed)
	case "tomp":
		tomp.ImportFeed(operator_feed)
	}
	operator_feed.LastImport = process.ProcessNewData(operator_feed.ImportStrategy, operator_feed.LastImport, newBikes)
}

// load feeds from database.
func loadFeeds() {

}
