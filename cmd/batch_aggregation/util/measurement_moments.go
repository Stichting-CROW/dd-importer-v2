package util

import "time"

func GetDefaultMeasrurementMoments(date time.Time) []time.Time {
	return []time.Time{
		time.Date(date.Year(), date.Month(), date.Day(), 3, 30, 0, 0, time.Local),
		time.Date(date.Year(), date.Month(), date.Day(), 8, 0, 0, 0, time.Local),
		time.Date(date.Year(), date.Month(), date.Day(), 10, 30, 0, 0, time.Local),
		time.Date(date.Year(), date.Month(), date.Day(), 14, 0, 0, 0, time.Local),
		time.Date(date.Year(), date.Month(), date.Day(), 17, 0, 0, 0, time.Local),
		time.Date(date.Year(), date.Month(), date.Day(), 21, 0, 0, 0, time.Local),
	}
}
