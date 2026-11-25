package process

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
)

// load feeds from database.
func LoadFeeds(oldFeeds []feed.Feed, db *sqlx.DB) []feed.Feed {
	log.Print("Sync new feeds")
	newFeeds := queryNewFeeds(db)
	for index, newFeed := range newFeeds {
		oldFeed := lookUpFeedID(oldFeeds, newFeed.ID)
		newFeeds[index].LastImport = oldFeed.LastImport
		newFeeds[index].NumberOfPulls = oldFeed.NumberOfPulls
		newFeeds[index].OAuth2Credentials.AccessToken = oldFeed.OAuth2Credentials.AccessToken
		newFeeds[index].OAuth2Credentials.ExpireTime = oldFeed.OAuth2Credentials.ExpireTime
		newFeeds[index].OAuth2CredentialsBasicAuth.AccessToken = oldFeed.OAuth2CredentialsBasicAuth.AccessToken
		newFeeds[index].OAuth2CredentialsBasicAuth.ExpireTime = oldFeed.OAuth2CredentialsBasicAuth.ExpireTime
		newFeeds[index].OAuth2CredentialsMoveyou.AccessToken = oldFeed.OAuth2CredentialsMoveyou.AccessToken
		newFeeds[index].OAuth2CredentialsMoveyou.ExpireTime = oldFeed.OAuth2CredentialsMoveyou.ExpireTime
	}
	return newFeeds
}

func LoadGeofencingFeeds(dataProcessor DataProcessor) []feed.Feed {
	return queryGeofencingFeeds(dataProcessor)
}

func LoadTripFeeds(dataProcessor DataProcessor) []feed.Feed {
	return queryTripFeeds(dataProcessor)
}

func lookUpFeedID(oldData []feed.Feed, ID int) feed.Feed {
	for _, feed := range oldData {
		if feed.ID == ID {
			return feed
		}
	}
	return feed.Feed{}
}

func queryNewFeeds(db *sqlx.DB) []feed.Feed {
	stmt := `SELECT feed_id, feeds.system_id, feed_url, 
		feed_type, import_strategy, authentication, request_headers,
		default_vehicle_type, form_factor, trip_import_start_date
		FROM feeds
		LEFT JOIN vehicle_type
		ON default_vehicle_type = vehicle_type_id
		WHERE feeds.data_type = 'vehicles'
		AND feeds.is_active = true
		ORDER BY feed_id
	`
	rows, err := db.Queryx(stmt)
	if err != nil {
		log.Print(err)
	}

	feeds := serializeFeeds(rows)
	log.Printf("Feeds opnieuw geïmporteerd, op dit moment zijn er %d feeds actief.", len(feeds))
	return feeds
}

func queryGeofencingFeeds(dataProcessor DataProcessor) []feed.Feed {
	stmt := `SELECT feed_id, feeds.system_id, feed_url, 
		feed_type, import_strategy, authentication, request_headers,
		default_vehicle_type, form_factor, trip_import_start_date
		FROM feeds
		LEFT JOIN vehicle_type
		ON default_vehicle_type = vehicle_type_id
		WHERE feeds.data_type = 'service_areas'
		AND feeds.is_active = true
		ORDER BY feed_id
	`
	rows, err := dataProcessor.DB.Queryx(stmt)
	if err != nil {
		log.Print(err)
	}

	return serializeFeeds(rows)
}

func queryTripFeeds(dataProcessor DataProcessor) []feed.Feed {
	stmt := `SELECT feed_id, feeds.system_id, feed_url, 
		feed_type, import_strategy, authentication, request_headers,
		default_vehicle_type, form_factor, trip_import_start_date
		FROM feeds
		LEFT JOIN vehicle_type
		ON default_vehicle_type = vehicle_type_id
		WHERE feeds.data_type = 'trips'
		AND feeds.is_active = true
		ORDER BY feed_id
	`
	rows, err := dataProcessor.DB.Queryx(stmt)
	if err != nil {
		log.Print(err)
	}

	return serializeFeeds(rows)
}

func serializeFeeds(rows *sqlx.Rows) []feed.Feed {
	feeds := []feed.Feed{}
	for rows.Next() {
		newFeed := feed.Feed{}
		authentication := []byte{}
		requestHeaders := []byte{}
		err := rows.Scan(&newFeed.ID, &newFeed.OperatorID,
			&newFeed.Url, &newFeed.Type, &newFeed.ImportStrategy,
			&authentication, &requestHeaders,
			&newFeed.DefaultVehicleType, &newFeed.DefaultFormFactor,
			&newFeed.TripImportStartDate)
		if err != nil {
			log.Print(err)
		}
		if err := feed.ParseAuthentication(&newFeed, authentication); err != nil {
			log.Printf("[%s_%d] skipping feed, invalid authentication: %v", newFeed.OperatorID, newFeed.ID, err)
			continue
		}
		json.Unmarshal([]byte(requestHeaders), &newFeed.RequestHeaders)
		feeds = append(feeds, newFeed)
	}
	return feeds
}
