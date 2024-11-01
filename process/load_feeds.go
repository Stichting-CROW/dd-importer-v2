package process

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/auth"
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
		newFeeds[index].OAuth2CredentialsGosharing.AccessToken = oldFeed.OAuth2CredentialsGosharing.AccessToken
		newFeeds[index].OAuth2CredentialsGosharing.ExpireTime = oldFeed.OAuth2CredentialsGosharing.ExpireTime
		newFeeds[index].OAuth2CredentialsBolt.AccessToken = oldFeed.OAuth2CredentialsBolt.AccessToken
		newFeeds[index].OAuth2CredentialsBolt.ExpireTime = oldFeed.OAuth2CredentialsBolt.ExpireTime
		newFeeds[index].OAuth2CredentialsMoveyou.AccessToken = oldFeed.OAuth2CredentialsMoveyou.AccessToken
		newFeeds[index].OAuth2CredentialsMoveyou.ExpireTime = oldFeed.OAuth2CredentialsMoveyou.ExpireTime
	}
	return newFeeds
}

func LoadGeofencingFeeds(dataProcessor DataProcessor) []feed.Feed {
	return queryGeofencingFeeds(dataProcessor)
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
		feed_type, import_strategy, authentication, last_time_updated, request_headers,
		default_vehicle_type, form_factor
		FROM feeds
		LEFT JOIN vehicle_type
		ON default_vehicle_type = vehicle_type_id
		WHERE feeds.import_vehicles = true
		AND feeds.is_active = true
		ORDER BY feed_id
	`
	rows, err := db.Queryx(stmt)
	if err != nil {
		log.Print(err)
	}

	return serializeFeeds(rows)
}

func queryGeofencingFeeds(dataProcessor DataProcessor) []feed.Feed {
	stmt := `SELECT feed_id, feeds.system_id, feed_url, 
		feed_type, import_strategy, authentication, last_time_updated, request_headers,
		default_vehicle_type, form_factor
		FROM feeds
		LEFT JOIN vehicle_type
		ON default_vehicle_type = vehicle_type_id
		WHERE feeds.import_service_area = true
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
	case "oauth2-dott":
		var config auth.DottAuthConfig

		res, _ := json.Marshal(result["OAuth2Credentials"])
		err := json.Unmarshal(res, &config)
		if err != nil {
			log.Print("Invalid config DOTT auth")
		}
		newFeed.OAuth2CredentialsDott.OauthTokenBody = config
	}

	return newFeed
}
