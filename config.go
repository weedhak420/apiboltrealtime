package main

// GetLocationsByPriority returns locations filtered by priority level
func GetLocationsByPriority(priority int) []Location {
	var filtered []Location
	for _, loc := range AllLocations {
		if loc.Priority == priority {
			filtered = append(filtered, loc)
		}
	}
	return filtered
}

// GetLocationsByType returns locations filtered by type
func GetLocationsByType(locType string) []Location {
	var filtered []Location
	for _, loc := range AllLocations {
		if loc.Type == locType {
			filtered = append(filtered, loc)
		}
	}
	return filtered
}

// GetLocationStats returns statistics about locations
func GetLocationStats() map[string]interface{} {
	byPriority := make(map[int]int)
	byType := make(map[string]int)

	for _, loc := range AllLocations {
		byPriority[loc.Priority]++
		byType[loc.Type]++
	}

	return map[string]interface{}{
		"total":       len(AllLocations),
		"by_priority": byPriority,
		"by_type":     byType,
	}
}
