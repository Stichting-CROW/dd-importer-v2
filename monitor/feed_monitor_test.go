package monitor

import (
	"database/sql"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 25, 12, 30, 0, 0, time.UTC)

// at returns the time a number of minutes before testNow.
func at(minutesAgo int) sql.NullTime {
	return sql.NullTime{Time: testNow.Add(-time.Duration(minutesAgo) * time.Minute), Valid: true}
}

func TestScheduledImportState(t *testing.T) {
	threshold := 150 * time.Minute
	never := sql.NullTime{}

	tests := []struct {
		name          string
		attempted     sql.NullTime
		succeeded     sql.NullTime
		failingSince  sql.NullTime
		wantState     importState
		wantDownSince time.Time
	}{
		{"never attempted", never, never, never, importStateUnknown, time.Time{}},
		{"last attempt succeeded", at(30), at(30), never, importStateUp, time.Time{}},
		{"one failed run", at(30), at(90), at(30), importStateUnknown, time.Time{}},
		{"two failed runs, within threshold", at(30), at(149), at(90), importStateUnknown, time.Time{}},
		{"two failed runs, past threshold", at(30), at(151), at(90), importStateDown, at(90).Time},
		{"failing for a day", at(30), at(1500), at(1440), importStateDown, at(1440).Time},
		{"never succeeded, first failures", at(30), never, at(90), importStateUnknown, time.Time{}},
		{"never succeeded, past threshold", at(30), never, at(151), importStateDown, at(151).Time},
		{"importer not running, feed was fine", at(151), at(151), never, importStateUnknown, time.Time{}},
		{"importer not running, feed was failing", at(151), at(600), at(540), importStateUnknown, time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feed := FeedStatus{
				DataType:                    dataTypeTrips,
				IsActive:                    true,
				LastTimeImportAttempted:     tt.attempted,
				LastTimeSuccesfullyImported: tt.succeeded,
				ImportFailingSince:          tt.failingSince,
			}
			state, downSince := scheduledImportState(feed, testNow, threshold)
			if state != tt.wantState {
				t.Errorf("state = %v, want %v", state, tt.wantState)
			}
			if !downSince.Equal(tt.wantDownSince) {
				t.Errorf("downSince = %v, want %v", downSince, tt.wantDownSince)
			}
		})
	}
}

func TestLatestImportAttempt(t *testing.T) {
	feeds := []FeedStatus{
		{DataType: dataTypeTrips, IsActive: true, LastTimeImportAttempted: at(200)},
		{DataType: dataTypeTrips, IsActive: true, LastTimeImportAttempted: at(40)},
		{DataType: dataTypeTrips, IsActive: true},
		{DataType: dataTypeTrips, IsActive: false, LastTimeImportAttempted: at(5)},
		{DataType: dataTypeVehicles, IsActive: true, LastTimeImportAttempted: at(1)},
	}

	latest, ok := latestImportAttempt(feeds, dataTypeTrips)
	if !ok || !latest.Equal(at(40).Time) {
		t.Errorf("latestImportAttempt(trips) = %v, %v, want %v, true", latest, ok, at(40).Time)
	}

	if _, ok := latestImportAttempt(feeds, dataTypeServiceAreas); ok {
		t.Errorf("latestImportAttempt(service_areas) found an attempt, want none")
	}
}

func TestParseDataTypes(t *testing.T) {
	if got := parseDataTypes(""); len(got) != 0 {
		t.Errorf("parseDataTypes(\"\") = %v, want empty", got)
	}

	got := parseDataTypes("trips, service_areas,")
	if len(got) != 2 || !got[dataTypeTrips] || !got[dataTypeServiceAreas] {
		t.Errorf("parseDataTypes() = %v, want trips and service_areas", got)
	}
}

func TestDataTypeLine(t *testing.T) {
	for _, dataType := range []string{"", dataTypeVehicles} {
		if got := dataTypeLine(dataType); got != "" {
			t.Errorf("dataTypeLine(%q) = %q, want no line", dataType, got)
		}
	}
	if got, want := dataTypeLine(dataTypeTrips), "<b>Data type:</b> trips\n"; got != want {
		t.Errorf("dataTypeLine(trips) = %q, want %q", got, want)
	}
}
