package main

import (
	"deelfietsdashboard-importer/cmd/batch_aggregation/indicators"
	"testing"
	"time"
)

func mustIndicators(t *testing.T, textIDs ...string) []indicators.Indicator {
	t.Helper()
	result := make([]indicators.Indicator, 0, len(textIDs))
	for _, textID := range textIDs {
		indicator, ok := indicators.GetByTextID(textID)
		if !ok {
			t.Fatalf("unknown indicator %q", textID)
		}
		result = append(result, indicator)
	}
	return result
}

func TestNewAggregationWindowMonthlyMissingDays(t *testing.T) {
	runStart := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	runEnd := time.Date(2026, time.November, 19, 0, 0, 0, 0, time.UTC)

	window := newAggregationWindow(runStart, runEnd, mustIndicators(t,
		"count_ended_rentals_by_distance_and_time_period"))

	if window.runDaily {
		t.Fatal("expected daily processing to be disabled for monthly-only indicators")
	}
	if !window.runMonthly {
		t.Fatal("expected monthly processing to be enabled")
	}
	assertDate(t, "monthStart", window.monthStart, 2026, time.November, 1)
	assertDate(t, "monthEnd", window.monthEnd, 2026, time.November, 1)
}

func TestNewAggregationWindowRefreshesLatestMonthWithoutNewDailyData(t *testing.T) {
	runStart := time.Date(2026, time.November, 20, 0, 0, 0, 0, time.UTC)
	runEnd := time.Date(2026, time.November, 19, 0, 0, 0, 0, time.UTC)

	window := newAggregationWindow(runStart, runEnd, mustIndicators(t,
		"count_ended_rentals_by_real_distance_and_time_period"))

	if window.runDaily {
		t.Fatal("expected no daily processing when runStart is after runEnd")
	}
	if !window.runMonthly {
		t.Fatal("expected the latest month to be refreshed")
	}
	assertDate(t, "monthStart", window.monthStart, 2026, time.November, 1)
	assertDate(t, "monthEnd", window.monthEnd, 2026, time.November, 1)
}

func TestNewAggregationWindowCrossesMonthBoundary(t *testing.T) {
	runStart := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	runEnd := time.Date(2026, time.December, 2, 0, 0, 0, 0, time.UTC)

	window := newAggregationWindow(runStart, runEnd, mustIndicators(t,
		"rented_out_vehicles_available_same_day",
		"count_ended_rentals_by_duration_and_time_period"))

	if !window.runDaily {
		t.Fatal("expected daily processing to be enabled")
	}
	if !window.runMonthly {
		t.Fatal("expected monthly processing to be enabled")
	}
	assertDate(t, "monthStart", window.monthStart, 2026, time.November, 1)
	assertDate(t, "monthEnd", window.monthEnd, 2026, time.December, 1)
}

func TestNewAggregationWindowNormalizesMixedLocations(t *testing.T) {
	cet := time.FixedZone("CET", 60*60)
	runStart := time.Date(2026, time.November, 20, 0, 0, 0, 0, time.UTC)
	runEnd := time.Date(2026, time.November, 20, 4, 0, 0, 0, cet)

	window := newAggregationWindow(runStart, runEnd, mustIndicators(t,
		"count_rentals_per_day",
		"count_ended_rentals_by_distance_and_time_period"))

	if !window.runDaily {
		t.Fatal("expected daily processing to be enabled")
	}
	if !window.runMonthly {
		t.Fatal("expected monthly processing to be enabled")
	}
	if window.monthStart.After(window.monthEnd) {
		t.Fatalf("monthStart %s is after monthEnd %s", window.monthStart, window.monthEnd)
	}
	if got := window.monthStart.Format("2006-01-02"); got != "2026-11-01" {
		t.Fatalf("monthStart = %s, want 2026-11-01", got)
	}
	if got := window.monthEnd.Format("2006-01-02"); got != "2026-11-01" {
		t.Fatalf("monthEnd = %s, want 2026-11-01", got)
	}
}

func TestNewAggregationWindowDailyOnly(t *testing.T) {
	runStart := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	runEnd := time.Date(2026, time.November, 19, 0, 0, 0, 0, time.UTC)

	window := newAggregationWindow(runStart, runEnd, mustIndicators(t,
		"rented_out_vehicles_available_same_day"))

	if !window.runDaily {
		t.Fatal("expected daily processing to be enabled")
	}
	if window.runMonthly {
		t.Fatal("expected monthly processing to be disabled")
	}
	assertDate(t, "monthStart", window.monthStart, 2026, time.November, 1)
	assertDate(t, "monthEnd", window.monthEnd, 2026, time.November, 1)
}

func assertDate(t *testing.T, name string, got time.Time, year int, month time.Month, day int) {
	t.Helper()
	want := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("%s = %s, want %s", name, got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}
