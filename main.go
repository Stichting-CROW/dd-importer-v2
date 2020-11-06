package main

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/gbfs"
	"deelfietsdashboard-importer/feed/tomp"
	"deelfietsdashboard-importer/process"
	"encoding/json"
	"log"
	"sync"
	"time"
)

func main() {
	feeds := []feed.Feed{}
	dataProcessor := process.InitDataProcessor()

	// Start processing of events in background.
	go dataProcessor.EventProcessor()

	importLoop(feeds, dataProcessor)
}

func importLoop(feeds []feed.Feed, dataProcessor process.DataProcessor) {
	var waitGroup sync.WaitGroup

	lastTimeUpdateFeedConfig := time.Time{}
	for {
		if time.Now().Sub(lastTimeUpdateFeedConfig) >= time.Minute*1 {
			lastTimeUpdateFeedConfig = time.Now()
			feeds = loadFeeds(feeds, dataProcessor)
		}

		startImport := time.Now()
		for index, _ := range feeds {
			waitGroup.Add(1)
			go importFeed(&feeds[index], &waitGroup, dataProcessor)
		}
		waitGroup.Wait()
		importDuration := time.Now().Sub(startImport)
		log.Printf("All imports took %v", importDuration)
		if importDuration.Seconds() <= 30 {
			time.Sleep(time.Second*30 - importDuration)
		}
	}
}

func importFeed(operator_feed *feed.Feed, waitGroup *sync.WaitGroup, dataProcessor process.DataProcessor) {
	defer waitGroup.Done()
	var newBikes []feed.Bike
	switch operator_feed.Type {
	case "gbfs":
		newBikes = gbfs.ImportFeed(operator_feed)
	case "tomp":
		newBikes = tomp.ImportFeed(operator_feed)
	}
	operator_feed.LastImport = dataProcessor.ProcessNewData(operator_feed.ImportStrategy, operator_feed.LastImport, newBikes).CurrentBikesInFeed
}

// load feeds from database.
func loadFeeds(oldFeeds []feed.Feed, dataProcessor process.DataProcessor) []feed.Feed {
	log.Print("Sync new feeds")
	newFeeds := queryNewFeeds(dataProcessor)
	for index, newFeed := range newFeeds {
		oldFeed := lookUpFeedID(oldFeeds, newFeed.ID)
		newFeeds[index].LastImport = oldFeed.LastImport
		newFeeds[index].NumberOfPulls = oldFeed.NumberOfPulls
	}
	return newFeeds

}

func lookUpFeedID(oldData []feed.Feed, ID int) feed.Feed {
	for _, feed := range oldData {
		if feed.ID == ID {
			return feed
		}
	}
	return feed.Feed{}

}

func queryNewFeeds(dataProcessor process.DataProcessor) []feed.Feed {
	stmt := `SELECT feed_id, system_id, feed_url, 
		feed_type, import_strategy, authentication, last_time_updated
		FROM feeds
		ORDER BY feed_id
	`
	rows, err := dataProcessor.DB.Queryx(stmt)
	if err != nil {
		log.Print(err)
	}

	feeds := []feed.Feed{}
	for rows.Next() {
		newFeed := feed.Feed{}
		authentication := []byte{}
		rows.Scan(&newFeed.ID, &newFeed.OperatorID, &newFeed.Url, &newFeed.Type, &newFeed.ImportStrategy, &authentication, &newFeed.LastTimeUpdated)
		newFeed = parseAuthentication(newFeed, authentication)
		feeds = append(feeds, newFeed)
	}
	return feeds
}

func parseAuthentication(newFeed feed.Feed, data []byte) feed.Feed {
	var result map[string]string
	json.Unmarshal([]byte(data), &result)
	newFeed.ApiKeyName = result["ApiKeyName"]
	newFeed.ApiKey = result["ApiKey"]
	return newFeed
}
