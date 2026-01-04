package util

import "log"

func GetNumericIndicatorID(measurementEnum string) int {
	switch measurementEnum {
	case "count_vehicles_in_public_space":
		return 1
	case "count_vehicles_in_public_space_longer_then_1_days":
		return 2
	case "count_vehicles_in_public_space_longer_then_3_days":
		return 3
	case "count_vehicles_in_public_space_longer_then_7_days":
		return 4
	case "count_vehicles_in_public_space_longer_then_14_days":
		return 5
	default:
		log.Fatalf("Unknown measurement enum: %s", measurementEnum)
		return -1
	}
}
