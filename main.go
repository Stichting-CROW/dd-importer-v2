package main

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/feed_status"
	"deelfietsdashboard-importer/feed/gbfs"
	"deelfietsdashboard-importer/feed/mds"
	mdsv2 "deelfietsdashboard-importer/feed/mds-v2"
	"deelfietsdashboard-importer/feed/tomp"
	"deelfietsdashboard-importer/monitor"
	"deelfietsdashboard-importer/process"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
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
	lastCleanupDate := time.Time{}
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

		now := time.Now()
		todayAt2 := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
		if firstImport || (now.After(todayAt2) && lastCleanupDate.Before(todayAt2)) {
			firstImport = false
			log.Print("Running cleanup:")
			events := cleanup(feeds, dataProcessor)
			dataProcessor.EventChan <- events
			lastCleanupDate = time.Now()
			importDuration = time.Since(startImport)
			log.Printf("Cleanup took %v, %d corrected events", importDuration, len(events))
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
	start := time.Now()
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
	if time.Since(start).Seconds() > 7 {
		log.Printf("[%s_%d] %s import took %.2f seconds, which is quite long, investigate if this can be improved", operator_feed.OperatorID, operator_feed.ID, operator_feed.Type, time.Since(start).Seconds())
	}

	// keobike en gosharing gaan fout
	if operator_feed.DefaultVehicleType != nil {
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
	log.Print("Wait until all events are processed.")
	for len(dataProcessor.EventChan) > 0 {
		time.Sleep(100 * time.Millisecond)
	}
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
	counter := 0
	logMsgs := ""
	for rows.Next() {
		counter += 1
		event := process.Event{}
		var bikeID string
		err := rows.Scan(&event.OperatorID, &bikeID, &event.RelatedParkEventID)
		event.Bike.SystemID = strings.Split(bikeID, ":")[0]
		event.Bike.BikeID = strings.Split(bikeID, ":")[1]
		if err != nil {
			log.Print(err)
		}
		if _, ok := bikeIDsInFeed[bikeID]; !ok {
			logMsg := fmt.Sprintf("bike_id=%s, operator_id=%s, park_event_id=%d", event.Bike.BikeID, event.Bike.SystemID, event.RelatedParkEventID)
			log.Printf("Correcting checkout: %s", logMsg)
			logMsgs += logMsg + "\n"
			event.EventType = "correcting_check_out"
			event.Timestamp = time.Now()
			events = append(events, event)
		}
	}

	log.Printf("%d open park_events in database during cleanup.", counter)

	notifier, err := monitor.NewTelegramNotifier()
	if err != nil {
		log.Print(err)
	}
	if logMsgs != "" {
		notifier.SendAlert(fmt.Sprintf("Corrected checkouts during cleanup: \n%s", logMsgs))
	}

	return events
}

func getAllParkedBikesFromDatabase(dataProcessor process.DataProcessor, operators map[string]bool) *sqlx.Rows {
	activeOperators := []string{}
	for key := range operators {
		activeOperators = append(activeOperators, key)
	}
	log.Printf("Getting all parked bikes from database for operators: %s", strings.Join(activeOperators, ","))

	stmt := `SELECT system_id, bike_id, park_event_id 
		FROM park_events 
		WHERE end_time is null 
		AND system_id = ANY($1);
	`
	rows, err := dataProcessor.DB.Queryx(stmt, pq.Array(activeOperators))
	if err != nil {
		log.Print(err)
	}
	return rows
}
