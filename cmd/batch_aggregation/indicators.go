package main

import (
	"database/sql"
	"log"
)

type Indicator struct {
	ID          int
	TextID      string
	Description string
}

var textToNumericIndicatorID map[string]int

func getIndicators() []Indicator {
	return []Indicator{
		{
			ID:          1,
			TextID:      "count_vehicles_in_public_space",
			Description: "Het aantal onverhuurde voertuigen.",
		},
		{
			ID:          2,
			TextID:      "count_vehicles_in_public_space_longer_then_1_days",
			Description: "Elke dag om 3:30 uur wordt bepaald hoeveel onverhuurde voertuigen in de openbare ruimte een parkeerduur hebben langer dan 1 dag.",
		},
		{
			ID:          3,
			TextID:      "count_vehicles_in_public_space_longer_then_3_days",
			Description: "Elke dag om 3:30 uur wordt bepaald hoeveel onverhuurde voertuigen in de openbare ruimte een parkeerduur hebben langer dan 3 dagen.",
		},
		{
			ID:          4,
			TextID:      "count_vehicles_in_public_space_longer_then_7_days",
			Description: "Elke dag om 3:30 uur wordt bepaald hoeveel onverhuurde voertuigen in de openbare ruimte een parkeerduur hebben langer dan 7 dagen.",
		},
		{
			ID:          5,
			TextID:      "count_vehicles_in_public_space_longer_then_14_days",
			Description: "Elke dag om 3:30 uur wordt bepaald hoeveel onverhuurde voertuigen in de openbare ruimte een parkeerduur hebben langer dan 14 dagen.",
		},
		{
			ID:          6,
			TextID:      "count_wrongly_parked_vehicles",
			Description: "Het aantal voertuigen dat verkeerd geparkeerd staat per dag in een no-parking zone.",
		},
	}
}

func initInitIndicators(db *sql.DB) {
	indicators := getIndicators()
	textToNumericIndicatorID = make(map[string]int)
	for _, indicator := range indicators {
		textToNumericIndicatorID[indicator.TextID] = indicator.ID
	}
	insertIndicators(db, indicators)
}

func GetNumericIndicatorID(measurementEnum string) int {
	id, exists := textToNumericIndicatorID[measurementEnum]
	if !exists {
		log.Fatalf("Indicator text ID %s not found", measurementEnum)
	}
	return id
}

func insertIndicators(db *sql.DB, indicators []Indicator) string {
	stmt := `
	INSERT INTO indicators (id, text_id, description, created_at)
	VALUES ($1, $2, $3, NOW())
	ON CONFLICT (id) DO NOTHING;
	`
	for _, indicator := range indicators {
		_, err := db.Exec(stmt, indicator.ID, indicator.TextID, indicator.Description)
		if err != nil {
			log.Fatal(err)
		}
	}
	return "Indicators inserted successfully."
}
