package main

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/gbfs"
	"deelfietsdashboard-importer/feed/mds"
	"deelfietsdashboard-importer/feed/tomp"
	"deelfietsdashboard-importer/process"
	"encoding/json"
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
			feeds = loadFeeds(feeds, dataProcessor)
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
		newBikes = gbfs.ImportFullFeed(operator_feed, dataProcessor)
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

// load feeds from database.
func loadFeeds(oldFeeds []feed.Feed, dataProcessor process.DataProcessor) []feed.Feed {
	log.Print("Sync new feeds")
	newFeeds := queryNewFeeds(dataProcessor)
	for index, newFeed := range newFeeds {
		oldFeed := lookUpFeedID(oldFeeds, newFeed.ID)
		newFeeds[index].LastImport = oldFeed.LastImport
		newFeeds[index].NumberOfPulls = oldFeed.NumberOfPulls
		newFeeds[index].OAuth2Credentials.AccessToken = oldFeed.OAuth2Credentials.AccessToken
		newFeeds[index].OAuth2Credentials.ExpireTime = oldFeed.OAuth2Credentials.ExpireTime
		newFeeds[index].OAuth2CredentialsGosharing.AccessToken = oldFeed.OAuth2CredentialsGosharing.AccessToken
		newFeeds[index].OAuth2CredentialsGosharing.ExpireTime = oldFeed.OAuth2CredentialsGosharing.ExpireTime
		newFeeds[index].OAuth2CredentialsBolt.AccessToken = oldFeed.OAuth2CredentialsBolt.AccessToken
		newFeeds[index].OAuth2CredentialsBolt.ExpireTime = oldFeed.OAuth2CredentialsBolt.ExpireTime
		newFeeds[index].OAuth2CredentialsMoveyou.AccessToken = oldFeed.OAuth2CredentialsMoveyou.AccessToken
		newFeeds[index].OAuth2CredentialsMoveyou.ExpireTime = oldFeed.OAuth2CredentialsMoveyou.ExpireTime
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
	stmt := `SELECT feed_id, feeds.system_id, feed_url, 
		feed_type, import_strategy, authentication, last_time_updated, request_headers,
		default_vehicle_type, form_factor
		FROM feeds
		LEFT JOIN vehicle_type
		ON default_vehicle_type = vehicle_type_id
		WHERE feeds.is_active = true
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
		requestHeaders := []byte{}
		rows.Scan(&newFeed.ID, &newFeed.OperatorID,
			&newFeed.Url, &newFeed.Type, &newFeed.ImportStrategy,
			&authentication, &newFeed.LastTimeUpdated, &requestHeaders,
			&newFeed.DefaultVehicleType, &newFeed.DefaultFormFactor)
		// Tijdelijk filter voor testen.
		newFeed = parseAuthentication(newFeed, authentication)
		json.Unmarshal([]byte(requestHeaders), &newFeed.RequestHeaders)
		feeds = append(feeds, newFeed)
	}
	log.Printf("Feeds opnieuw geïmporteerd, op dit moment zijn er %d feeds actief.", len(feeds))
	return feeds
}

func parseAuthentication(newFeed feed.Feed, data []byte) feed.Feed {
	var result map[string]interface{}
	json.Unmarshal([]byte(data), &result)
	if _, ok := result["authentication_type"]; !ok {
		return newFeed
	}
	newFeed.AuthenticationType = result["authentication_type"].(string)
	switch newFeed.AuthenticationType {
	case "token":
		newFeed.ApiKeyName = result["ApiKeyName"].(string)
		newFeed.ApiKey = result["ApiKey"].(string)
	case "oauth2":
		newFeed.OAuth2Credentials.OauthTokenBody = result["OAuth2Credentials"].(map[string]interface{})
		newFeed.OAuth2Credentials.TokenURL = result["TokenURL"].(string)
	case "oauth2-gosharing":
		newFeed.OAuth2CredentialsGosharing.OauthTokenBody = result["OAuth2Credentials"].(map[string]interface{})
		newFeed.OAuth2CredentialsGosharing.TokenURL = result["TokenURL"].(string)
	case "oauth2-bolt":
		newFeed.OAuth2CredentialsBolt.OauthTokenBody = result["OAuth2Credentials"].(map[string]interface{})
		newFeed.OAuth2CredentialsBolt.TokenURL = result["TokenURL"].(string)
	case "oauth2-moveyou":
		newFeed.OAuth2CredentialsMoveyou.OauthTokenBody = result["OAuth2Credentials"].(map[string]interface{})
		newFeed.OAuth2CredentialsMoveyou.TokenURL = result["TokenURL"].(string)
	}

	return newFeed
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
