package main

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/feed_status"
	"deelfietsdashboard-importer/feed/gbfs"
	"deelfietsdashboard-importer/feed/mds"
	mdsv2 "deelfietsdashboard-importer/feed/mds-v2"
	"deelfietsdashboard-importer/feed/tomp"
	"deelfietsdashboard-importer/process"
	"log"
	"os"
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
		import_succesfull_chan := make(chan int, 1000)

		// This can be improved in the future in case we also store all is_disabled and is_reserved states in the database.
		all_vehicles := make(chan []feed.Bike, 1000)
		if time.Since(lastTimeUpdateFeedConfig) >= time.Minute*1 {
			lastTimeUpdateFeedConfig = time.Now()
			feeds = process.LoadFeeds(feeds, dataProcessor.DB)
			*dataProcessor.NumberOfFeedsActive = len(feeds)
		}

		startImport := time.Now()
		for index := range feeds {
			waitGroup.Add(1)
			go importFeed(&feeds[index], &waitGroup, dataProcessor, import_succesfull_chan, all_vehicles)
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

		close(import_succesfull_chan)
		var feeds_succesfully_imported []int

		for feed_id := range import_succesfull_chan {
			feeds_succesfully_imported = append(feeds_succesfully_imported, feed_id)
		}
		feed_status.UpdateLastTimeSuccesfullyImported(feeds_succesfully_imported, dataProcessor.DB)

		close(all_vehicles)
		// Only cache data when this salt is set.
		if os.Getenv("AVAILABLE_VEHICLES_ID_SALT") != "" {
			cacheAvailableVehicles(dataProcessor, all_vehicles)
		}

		if importDuration.Seconds() <= 30 {
			time.Sleep(time.Second*30 - importDuration)
		}

	}
}

func importFeed(operator_feed *feed.Feed, waitGroup *sync.WaitGroup, dataProcessor process.DataProcessor, import_succesfull chan int, vehicles chan []feed.Bike) {
	defer waitGroup.Done()
	var newBikes []feed.Bike
	switch operator_feed.Type {
	case "gbfs":
		newBikes = gbfs.ImportFeed(operator_feed)
	case "tomp":
		newBikes = tomp.ImportFeed(operator_feed)
	case "mds":
		newBikes = mds.ImportFeed(operator_feed)
	case "mds-v2":
		newBikes = mdsv2.ImportFeed(operator_feed)
	case "full_gbfs":
		newBikes = gbfs.ImportFullFeedVehicles(dataProcessor.DB, operator_feed)
	}

	// keobike en gosharing gaan fout
	if operator_feed.DefaultVehicleType != nil {
		log.Printf("Overrule default vehicle type %s %d", operator_feed.OperatorID, *operator_feed.DefaultVehicleType)

		newBikes = setDefaultInternalVehicleType(newBikes, *operator_feed.DefaultVehicleType, *operator_feed.DefaultFormFactor)
	}

	if len(newBikes) > 0 {
		import_succesfull <- operator_feed.ID
		vehicles <- newBikes
	}

	log.Printf("[%s_%d] %s import finished, %d vehicles in feed", operator_feed.OperatorID, operator_feed.ID, operator_feed.Type, len(newBikes))
	operator_feed.LastImport = dataProcessor.ProcessNewData(operator_feed.ImportStrategy, operator_feed.LastImport, newBikes).CurrentBikesInFeed
}

func setDefaultInternalVehicleType(bikes []feed.Bike, defaultType int, defaultFormFactor string) []feed.Bike {
	didOverride := 0
	for index := range bikes {
		if bikes[index].InternalVehicleID == nil {
			bikes[index].InternalVehicleID = &defaultType
			bikes[index].VehicleType = defaultFormFactor
			didOverride += 1
		}
	}
	if didOverride > 0 {
		log.Printf("Vehicle type is %d times overruled for feed of [%s]", didOverride, bikes[0].SystemID)
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
