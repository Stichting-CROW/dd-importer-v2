package indicators

import (
	"fmt"
	"strings"
	"time"
)

var DefaultFirstDay = time.Date(2019, 12, 31, 0, 0, 0, 0, time.UTC)

type Indicator struct {
	ID          int
	TextID      string
	Description string
	FirstDay    time.Time
}

var All = []Indicator{
	{
		ID:          1,
		TextID:      "count_vehicles_in_public_space",
		Description: "Het aantal onverhuurde voertuigen.",
		FirstDay:    DefaultFirstDay,
	},
	{
		ID:          2,
		TextID:      "count_vehicles_in_public_space_longer_then_1_days",
		Description: "Elke dag om 3:30 uur wordt bepaald hoeveel onverhuurde voertuigen in de openbare ruimte een parkeerduur hebben langer dan 1 dag.",
		FirstDay:    DefaultFirstDay,
	},
	{
		ID:          3,
		TextID:      "count_vehicles_in_public_space_longer_then_3_days",
		Description: "Elke dag om 3:30 uur wordt bepaald hoeveel onverhuurde voertuigen in de openbare ruimte een parkeerduur hebben langer dan 3 dagen.",
		FirstDay:    DefaultFirstDay,
	},
	{
		ID:          4,
		TextID:      "count_vehicles_in_public_space_longer_then_7_days",
		Description: "Elke dag om 3:30 uur wordt bepaald hoeveel onverhuurde voertuigen in de openbare ruimte een parkeerduur hebben langer dan 7 dagen.",
		FirstDay:    DefaultFirstDay,
	},
	{
		ID:          5,
		TextID:      "count_vehicles_in_public_space_longer_then_14_days",
		Description: "Elke dag om 3:30 uur wordt bepaald hoeveel onverhuurde voertuigen in de openbare ruimte een parkeerduur hebben langer dan 14 dagen.",
		FirstDay:    DefaultFirstDay,
	},
	{
		ID:          6,
		TextID:      "count_wrongly_parked_vehicles",
		Description: "Het aantal voertuigen dat verkeerd geparkeerd staat per dag in een no-parking zone.",
		FirstDay:    DefaultFirstDay,
	},
}

var (
	byTextID map[string]Indicator
	byID     map[int]Indicator
)

func init() {
	byTextID = make(map[string]Indicator, len(All))
	byID = make(map[int]Indicator, len(All))
	for _, indicator := range All {
		byTextID[indicator.TextID] = indicator
		byID[indicator.ID] = indicator
	}
}

func GetNumericIndicatorID(textID string) (int, error) {
	indicator, ok := byTextID[textID]
	if !ok {
		return 0, fmt.Errorf("unknown indicator text ID %q", textID)
	}
	return indicator.ID, nil
}

func GetByTextID(textID string) (Indicator, bool) {
	indicator, ok := byTextID[textID]
	return indicator, ok
}

func GetByID(id int) (Indicator, bool) {
	indicator, ok := byID[id]
	return indicator, ok
}

func TextIDs() []string {
	ids := make([]string, len(All))
	for i, indicator := range All {
		ids[i] = indicator.TextID
	}
	return ids
}

func IDs() []int32 {
	ids := make([]int32, len(All))
	for i, indicator := range All {
		ids[i] = int32(indicator.ID)
	}
	return ids
}

func IDsFromIndicators(indicators []Indicator) []int32 {
	ids := make([]int32, len(indicators))
	for i, indicator := range indicators {
		ids[i] = int32(indicator.ID)
	}
	return ids
}

func Resolve(textIDs string) ([]Indicator, error) {
	if strings.TrimSpace(textIDs) == "" {
		return nil, fmt.Errorf("indicator list is empty")
	}

	parts := strings.Split(textIDs, ",")
	resolved := make([]Indicator, 0, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		indicator, ok := GetByTextID(id)
		if !ok {
			return nil, fmt.Errorf("unknown indicator text ID %q (valid: %s)", id, strings.Join(TextIDs(), ", "))
		}
		resolved = append(resolved, indicator)
	}
	return resolved, nil
}

func EffectiveStartDate(indicator Indicator, requestedStart time.Time) time.Time {
	if requestedStart.Before(indicator.FirstDay) {
		return indicator.FirstDay
	}
	return requestedStart
}

func HasIndicator(selected []Indicator, textID string) bool {
	id, err := GetNumericIndicatorID(textID)
	if err != nil {
		return false
	}
	for _, indicator := range selected {
		if indicator.ID == id {
			return true
		}
	}
	return false
}

func IsSelectedOnDate(selected []Indicator, textID string, date time.Time) bool {
	id, err := GetNumericIndicatorID(textID)
	if err != nil {
		return false
	}
	for _, indicator := range selected {
		if indicator.ID == id && !date.Before(indicator.FirstDay) {
			return true
		}
	}
	return false
}

func IsSelectedForChunk(selected []Indicator, textID string, startDate, endDate time.Time) bool {
	id, err := GetNumericIndicatorID(textID)
	if err != nil {
		return false
	}
	for _, indicator := range selected {
		if indicator.ID == id && !endDate.Before(indicator.FirstDay) {
			return true
		}
	}
	return false
}
