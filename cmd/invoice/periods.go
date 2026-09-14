package main

import (
	"fmt"
	"time"
)

// servicePeriods validates all month arguments before any archive is reserved.
func servicePeriods(date time.Time, period, from, to string) ([]string, error) {
	if from == "" && to == "" {
		if period == "" {
			period = date.Format("2006-01")
		}
		if _, err := parseMonth("-period", period); err != nil {
			return nil, err
		}
		return []string{period}, nil
	}
	if period != "" {
		return nil, fmt.Errorf("-period cannot be combined with -from or -to")
	}
	if from == "" || to == "" {
		return nil, fmt.Errorf("-from and -to must be provided together")
	}
	start, err := parseMonth("-from", from)
	if err != nil {
		return nil, err
	}
	end, err := parseMonth("-to", to)
	if err != nil {
		return nil, err
	}
	if start.After(end) {
		return nil, fmt.Errorf("-from must be before or equal to -to")
	}
	var periods []string
	for month := start; !month.After(end); month = month.AddDate(0, 1, 0) {
		periods = append(periods, month.Format("2006-01"))
	}
	return periods, nil
}

func parseMonth(flagName, value string) (time.Time, error) {
	month, err := time.Parse("2006-01", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s; use YYYY-MM: %w", flagName, err)
	}
	if month.Year() < 1 {
		return time.Time{}, fmt.Errorf("invalid %s; year must be between 0001 and 9999", flagName)
	}
	return month, nil
}
