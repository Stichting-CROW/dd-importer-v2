package feed

import (
	"deelfietsdashboard-importer/feed/auth"
	"log"
	"os"
	"strings"
	"time"
)

// debugEnabled turns on verbose output (such as raw API responses) when the
// DEBUG environment variable is set to 1, true or yes.
var debugEnabled = func() bool {
	switch strings.ToLower(os.Getenv("DEBUG")) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}()

type Feed struct {
	ID                         int
	OperatorID                 string
	DefaultVehicleType         *int
	DefaultFormFactor          *string
	Url                        string
	ApiKeyName                 string
	ApiKey                     string
	NumberOfPulls              int
	RequestHeaders             map[string]string
	Type                       string
	LastImport                 map[string]Bike
	ImportStrategy             string
	OAuth2Credentials          auth.OauthCredentials
	OAuth2CredentialsBasicAuth auth.OauthCredentialsBasicAuth
	OAuth2CredentialsMoveyou   auth.OauthCredentialsMoveyou
	OAuth2CredentialsDott      auth.OauthCredentialsDott
	AuthenticationType         string
	LastTimeUpdated            time.Time
}

// LogZeroRecords logs the full raw response for a request that returned no
// records, but only when the DEBUG environment variable is enabled.
func (f *Feed) LogZeroRecords(url string, body []byte) {
	if !debugEnabled {
		return
	}
	log.Printf("[%s_%d] 0 records for %s, full response: %s", f.OperatorID, f.ID, url, body)
}

type Bike struct {
	BikeID                string  `json:"bike_id"`
	Lat                   float64 `json:"lat"`
	Lon                   float64 `json:"lon"`
	IsReserved            bool    `json:"is_reserved"`
	IsDisabled            bool    `json:"is_disabled"`
	SystemID              string  `json:"system_id"`
	InternalVehicleID     *int    `json:"internal_vehicle_id,omitempty"`
	ExternalVehicleTypeID *string `json:"vehicle_type_id,omitempty"`
	VehicleType           string  `json:"vehicle_type,omitempty"`
}
