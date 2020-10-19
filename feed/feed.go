package feed

type Feed struct {
	OperatorID     string
	Url            string
	ApiKeyName     string
	ApiKey         string
	NumberOfPulls  int
	Type           string
	LastImport     map[string]Bike
	ImportStrategy string
}

type Bike struct {
	BikeID     string  `json:"bike_id"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	IsReserved bool    `json:"is_reserved"`
	IsDisabled bool    `json:"is_disabled"`
}
