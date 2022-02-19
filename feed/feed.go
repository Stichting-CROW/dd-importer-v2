package feed

import (
	"deelfietsdashboard-importer/feed/auth"
	"time"
)

type Feed struct {
	ID                 int
	OperatorID         string
	DefaultVehicleType *int
	Url                string
	ApiKeyName         string
	ApiKey             string
	NumberOfPulls      int
	RequestHeaders     map[string]string
	Type               string
	LastImport         map[string]Bike
	ImportStrategy     string
	OAuth2Credentials  auth.OauthCredentials
	AuthenticationType string
	LastTimeUpdated    time.Time
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
}
