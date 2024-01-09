package main

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/gbfs"
	"deelfietsdashboard-importer/feed/mds"
	"deelfietsdashboard-importer/feed/tomp"
	"deelfietsdashboard-importer/process"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

func main() {
	feeds := []feed.Feed{}
	dataProcessor := process.InitDataProcessor()

	//Start processing of events in background.
	go dataProcessor.EventProcessor()
	go dataProcessor.VehicleProcessor()

	importLoop(feeds, dataProcessor)
}

func importLoop(feeds []feed.Feed, dataProcessor process.DataProcessor) {
	var waitGroup sync.WaitGroup

	lastTimeUpdateFeedConfig := time.Time{}
	firstImport := true
	for {
		if time.Since(lastTimeUpdateFeedConfig) >= time.Minute*1 {
			lastTimeUpdateFeedConfig = time.Now()
			feeds = process.LoadFeeds(feeds, dataProcessor.DB)
			*dataProcessor.NumberOfFeedsActive = len(feeds)
		}

		startImport := time.Now()
		for index := range feeds {
			waitGroup.Add(1)
			go importFeed(&feeds[index], &waitGroup, dataProcessor)
		}
		waitGroup.Wait()
		importDuration := time.Since(startImport)
		log.Printf("All imports took %v", importDuration)
		if firstImport {
			firstImport = false
			log.Print("This is the first import run cleanup:")
			events := cleanup(feeds, dataProcessor)
			dataProcessor.EventChan <- events
			importDuration = time.Since(startImport)
			log.Printf("All imports including cleanup took %v", importDuration)
		}
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
	case "mds":
		newBikes = mds.ImportFeed(operator_feed)
	case "full_gbfs":
		newBikes = gbfs.ImportFullFeed(dataProcessor.DB, operator_feed)
	}
	// keobike en gosharing gaan fout
	if operator_feed.DefaultVehicleType != nil {
		newBikes = setDefaultInternalVehicleType(newBikes, *operator_feed.DefaultVehicleType, *operator_feed.DefaultFormFactor)
	}

	log.Printf("[%s] %s import finished, %d vehicles in feed", operator_feed.OperatorID, operator_feed.Type, len(newBikes))
	operator_feed.LastImport = dataProcessor.ProcessNewData(operator_feed.ImportStrategy, operator_feed.LastImport, newBikes).CurrentBikesInFeed
}

func setDefaultInternalVehicleType(bikes []feed.Bike, defaultType int, defaultFormFactor string) []feed.Bike {
	for index := range bikes {
		if bikes[index].InternalVehicleID == nil {
			bikes[index].InternalVehicleID = &defaultType
			bikes[index].VehicleType = defaultFormFactor
		}
	}
	return bikes
}

// This function checkOuts all the bikes that are not in a feed anymore when the program starts running.
func cleanup(feeds []feed.Feed, dataProcessor process.DataProcessor) []process.Event {
	log.Print("Wait 25s until all events are processed.")
	time.Sleep(time.Second * 25)
	events := []process.Event{}
	operators := map[string]bool{}
	bikeIDsInFeed := map[string]bool{}
	for _, feed := range feeds {
		if len(feed.LastImport) != 0 {
			operators[feed.OperatorID] = true
			for bikeId := range feed.LastImport {
				bikeIDsInFeed[feed.OperatorID+":"+bikeId] = true
			}
		} else {
			log.Printf("%s failed to import (or has at least 0 vehicles in it's endpoint).", feed.OperatorID)
		}
	}

	rows := getAllParkedBikesFromDatabase(dataProcessor, operators)
	for rows.Next() {
		event := process.Event{}
		err := rows.Scan(&event.OperatorID, &event.Bike.BikeID, &event.RelatedParkEventID)
		if err != nil {
			log.Print(err)
		}
		if _, ok := bikeIDsInFeed[event.Bike.BikeID]; !ok {
			log.Print("BikeID not found", event.Bike.BikeID)
			event.EventType = "correcting_check_out"
			event.Timestamp = time.Now()
			events = append(events, event)
		}

	}

	return events
}

func getAllParkedBikesFromDatabase(dataProcessor process.DataProcessor, operators map[string]bool) *sqlx.Rows {
	keys := []string{}
	for key := range operators {
		keys = append(keys, key)
	}
	activeOperators := strings.Join(keys, ",")

	stmt := `SELECT system_id, bike_id, park_event_id 
		FROM park_events 
		WHERE end_time is null 
		AND system_id IN ($1);
	`
	rows, err := dataProcessor.DB.Queryx(stmt, activeOperators)
	if err != nil {
		log.Print(err)
	}
	return rows
}
