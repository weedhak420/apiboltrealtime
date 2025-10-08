package main

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"
)

// Analytics data structures
type HourlyAnalytics struct {
	Hour  int     `json:"hour"`
	Count int     `json:"count"`
	Avg   float64 `json:"average"`
	Peak  int     `json:"peak"`
}

type DailyAnalytics struct {
	Date  string  `json:"date"`
	Count int     `json:"count"`
	Avg   float64 `json:"average"`
	Peak  int     `json:"peak"`
}

type WeeklyAnalytics struct {
	Week  string  `json:"week"`
	Count int     `json:"count"`
	Avg   float64 `json:"average"`
	Peak  int     `json:"peak"`
}

type PeakHours struct {
	PeakHour    int     `json:"peak_hour"`
	QuietHour   int     `json:"quiet_hour"`
	PeakCount   int     `json:"peak_count"`
	QuietCount  int     `json:"quiet_count"`
	PeakPercent float64 `json:"peak_percent"`
}

type LocationStats struct {
	Location    string    `json:"location"`
	Count       int       `json:"count"`
	SuccessRate float64   `json:"success_rate"`
	AvgResponse float64   `json:"avg_response_ms"`
	LastUpdate  time.Time `json:"last_update"`
}

// Get hourly analytics
func getHourlyAnalytics() ([]HourlyAnalytics, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT 
			HOUR(timestamp) as hour,
			COUNT(*) as count,
			AVG(1) as avg_count,
			MAX(1) as peak
		FROM vehicle_cache 
		WHERE created_at > DATE_SUB(NOW(), INTERVAL 24 HOUR)
		GROUP BY HOUR(timestamp)
		ORDER BY hour
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query hourly analytics: %v", err)
	}
	defer rows.Close()

	var analytics []HourlyAnalytics
	for rows.Next() {
		var a HourlyAnalytics
		err := rows.Scan(&a.Hour, &a.Count, &a.Avg, &a.Peak)
		if err != nil {
			log.Printf("⚠️ Failed to scan hourly analytics: %v", err)
			continue
		}
		analytics = append(analytics, a)
	}

	return analytics, nil
}

// Get daily analytics
func getDailyAnalytics() ([]DailyAnalytics, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as count,
			AVG(1) as avg_count,
			MAX(1) as peak
		FROM vehicle_cache 
		WHERE created_at > DATE_SUB(NOW(), INTERVAL 7 DAY)
		GROUP BY DATE(created_at)
		ORDER BY date DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily analytics: %v", err)
	}
	defer rows.Close()

	var analytics []DailyAnalytics
	for rows.Next() {
		var a DailyAnalytics
		err := rows.Scan(&a.Date, &a.Count, &a.Avg, &a.Peak)
		if err != nil {
			log.Printf("⚠️ Failed to scan daily analytics: %v", err)
			continue
		}
		analytics = append(analytics, a)
	}

	return analytics, nil
}

// Get weekly analytics
func getWeeklyAnalytics() ([]WeeklyAnalytics, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT 
			YEARWEEK(created_at) as week,
			COUNT(*) as count,
			AVG(1) as avg_count,
			MAX(1) as peak
		FROM vehicle_cache 
		WHERE created_at > DATE_SUB(NOW(), INTERVAL 4 WEEK)
		GROUP BY YEARWEEK(created_at)
		ORDER BY week DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query weekly analytics: %v", err)
	}
	defer rows.Close()

	var analytics []WeeklyAnalytics
	for rows.Next() {
		var a WeeklyAnalytics
		err := rows.Scan(&a.Week, &a.Count, &a.Avg, &a.Peak)
		if err != nil {
			log.Printf("⚠️ Failed to scan weekly analytics: %v", err)
			continue
		}
		analytics = append(analytics, a)
	}

	return analytics, nil
}

// Get peak hours analysis
func getPeakHours() (PeakHours, error) {
	if db == nil {
		return PeakHours{}, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT 
			HOUR(timestamp) as hour,
			COUNT(*) as count
		FROM vehicle_cache 
		WHERE created_at > DATE_SUB(NOW(), INTERVAL 24 HOUR)
		GROUP BY HOUR(timestamp)
		ORDER BY count DESC
		LIMIT 1
	`

	var peakHour, peakCount int
	err := db.QueryRow(query).Scan(&peakHour, &peakCount)
	if err != nil {
		return PeakHours{}, fmt.Errorf("failed to get peak hours: %v", err)
	}

	// Get quiet hour
	query = `
		SELECT 
			HOUR(timestamp) as hour,
			COUNT(*) as count
		FROM vehicle_cache 
		WHERE created_at > DATE_SUB(NOW(), INTERVAL 24 HOUR)
		GROUP BY HOUR(timestamp)
		ORDER BY count ASC
		LIMIT 1
	`

	var quietHour, quietCount int
	err = db.QueryRow(query).Scan(&quietHour, &quietCount)
	if err != nil {
		quietHour = 0
		quietCount = 0
	}

	peakPercent := 0.0
	if peakCount > 0 {
		peakPercent = float64(peakCount) / float64(peakCount+quietCount) * 100
	}

	return PeakHours{
		PeakHour:    peakHour,
		QuietHour:   quietHour,
		PeakCount:   peakCount,
		QuietCount:  quietCount,
		PeakPercent: peakPercent,
	}, nil
}

// Get location statistics
func getLocationStats() ([]LocationStats, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT 
			source_location,
			COUNT(*) as count,
			AVG(1) as success_rate,
			AVG(1) as avg_response,
			MAX(created_at) as last_update
		FROM vehicle_cache 
		WHERE created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR)
		GROUP BY source_location
		ORDER BY count DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query location stats: %v", err)
	}
	defer rows.Close()

	var stats []LocationStats
	for rows.Next() {
		var s LocationStats
		err := rows.Scan(&s.Location, &s.Count, &s.SuccessRate, &s.AvgResponse, &s.LastUpdate)
		if err != nil {
			log.Printf("⚠️ Failed to scan location stats: %v", err)
			continue
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// Record analytics data
func recordAnalyticsData(metricName string, value float64, metadata map[string]interface{}) error {
	if db == nil {
		return nil // Skip if database not available
	}

	// Convert metadata to JSON (simplified)
	_ = metadata // Use metadata if needed in the future

	_, err := db.Exec(`
		INSERT INTO performance_stats (metric_name, metric_value, timestamp) 
		VALUES (?, ?, NOW())
	`, metricName, value)

	if err != nil {
		log.Printf("⚠️ Failed to record analytics data: %v", err)
	}

	return nil
}

// Clean old analytics data
func cleanOldAnalyticsData() error {
	if db == nil {
		return nil
	}

	// Clean data older than 30 days
	_, err := db.Exec("DELETE FROM performance_stats WHERE timestamp < DATE_SUB(NOW(), INTERVAL 30 DAY)")
	if err != nil {
		log.Printf("⚠️ Failed to clean old analytics data: %v", err)
	}

	return nil
}

// History analytics types
type HistoryAnalyticsParams struct {
	Start, End    time.Time
	VehicleID     string
	BBox          *struct{ MinLat, MinLng, MaxLat, MaxLng float64 }
	Limit, Offset int
	Grid          float64
	StopMinSec    int
	StopMaxMoveM  float64
}

type VehicleSummary struct {
	VehicleID   string  `json:"vehicle_id"`
	Points      int     `json:"points"`
	DistanceKM  float64 `json:"distance_km"`
	AvgSpeedKMH float64 `json:"avg_speed_kmh"`
	Stops       int     `json:"stops"`
}

type HistoryAnalytics struct {
	Summary struct {
		Start           time.Time `json:"start"`
		End             time.Time `json:"end"`
		RecordCount     int       `json:"record_count"`
		VehicleCount    int       `json:"vehicle_count"`
		TotalDistanceKM float64   `json:"total_distance_km"`
		AvgSpeedKMH     float64   `json:"avg_speed_kmh"`
		StopCount       int       `json:"stop_count"`
	} `json:"summary"`
	BearingHistogram map[string]int   `json:"bearing_histogram"`
	Hotspots         []Hotspot        `json:"hotspots"`
	Vehicles         []VehicleSummary `json:"vehicles"`
}

// calculateDistanceHaversine calculates distance between two points in meters using Haversine formula
func calculateDistanceHaversine(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000 // Earth's radius in meters

	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// getBearingDirection converts bearing angle to cardinal direction
func getBearingDirection(bearing float64) string {
	// Normalize bearing to 0-360
	for bearing < 0 {
		bearing += 360
	}
	for bearing >= 360 {
		bearing -= 360
	}

	// Map to 8 cardinal directions
	if bearing >= 337.5 || bearing < 22.5 {
		return "N"
	} else if bearing >= 22.5 && bearing < 67.5 {
		return "NE"
	} else if bearing >= 67.5 && bearing < 112.5 {
		return "E"
	} else if bearing >= 112.5 && bearing < 157.5 {
		return "SE"
	} else if bearing >= 157.5 && bearing < 202.5 {
		return "S"
	} else if bearing >= 202.5 && bearing < 247.5 {
		return "SW"
	} else if bearing >= 247.5 && bearing < 292.5 {
		return "W"
	} else {
		return "NW"
	}
}

// getHistoryAnalytics computes comprehensive analytics from vehicle history
func getHistoryAnalytics(params HistoryAnalyticsParams) (*HistoryAnalytics, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Build base query
	baseQuery := `
		SELECT history_id, vehicle_id, lat, lng, bearing, category_name, timestamp, created_at
		FROM vehicle_history
		WHERE timestamp BETWEEN ? AND ?
	`

	args := []interface{}{params.Start, params.End}
	query := baseQuery

	// Add optional filters
	if params.VehicleID != "" {
		query += " AND vehicle_id = ?"
		args = append(args, params.VehicleID)
	}

	if params.BBox != nil {
		query += " AND lat BETWEEN ? AND ? AND lng BETWEEN ? AND ?"
		args = append(args, params.BBox.MinLat, params.BBox.MaxLat, params.BBox.MinLng, params.BBox.MaxLng)
	}

	// Add ordering and pagination
	query += " ORDER BY vehicle_id ASC, timestamp ASC LIMIT ? OFFSET ?"
	args = append(args, params.Limit, params.Offset)

	// Execute query
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query vehicle history: %v", err)
	}
	defer rows.Close()

	// Process records
	var records []HistoryRecord
	for rows.Next() {
		var record HistoryRecord
		var timestamp, createdAt time.Time

		err := rows.Scan(
			&record.HistoryID,
			&record.VehicleID,
			&record.Lat,
			&record.Lng,
			&record.Bearing,
			&record.CategoryName,
			&timestamp,
			&createdAt,
		)
		if err != nil {
			log.Printf("⚠️ Failed to scan history record: %v", err)
			continue
		}

		record.Timestamp = timestamp.Format("2006-01-02 15:04:05")
		record.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		records = append(records, record)
	}

	if len(records) == 0 {
		return &HistoryAnalytics{
			Summary: struct {
				Start           time.Time `json:"start"`
				End             time.Time `json:"end"`
				RecordCount     int       `json:"record_count"`
				VehicleCount    int       `json:"vehicle_count"`
				TotalDistanceKM float64   `json:"total_distance_km"`
				AvgSpeedKMH     float64   `json:"avg_speed_kmh"`
				StopCount       int       `json:"stop_count"`
			}{
				Start: params.Start,
				End:   params.End,
			},
			BearingHistogram: map[string]int{},
			Hotspots:         []Hotspot{},
			Vehicles:         []VehicleSummary{},
		}, nil
	}

	// Group records by vehicle
	vehicleGroups := make(map[string][]HistoryRecord)
	for _, record := range records {
		vehicleGroups[record.VehicleID] = append(vehicleGroups[record.VehicleID], record)
	}

	// Initialize analytics result
	analytics := &HistoryAnalytics{
		Summary: struct {
			Start           time.Time `json:"start"`
			End             time.Time `json:"end"`
			RecordCount     int       `json:"record_count"`
			VehicleCount    int       `json:"vehicle_count"`
			TotalDistanceKM float64   `json:"total_distance_km"`
			AvgSpeedKMH     float64   `json:"avg_speed_kmh"`
			StopCount       int       `json:"stop_count"`
		}{
			Start: params.Start,
			End:   params.End,
		},
		BearingHistogram: map[string]int{
			"N": 0, "NE": 0, "E": 0, "SE": 0,
			"S": 0, "SW": 0, "W": 0, "NW": 0,
		},
		Hotspots: []Hotspot{},
		Vehicles: []VehicleSummary{},
	}

	// Process each vehicle
	vehicleSummaries := []VehicleSummary{}
	hotspotMap := make(map[string]map[string]bool) // gridKey -> vehicleID set
	totalDistance := 0.0
	totalTime := 0.0
	totalStops := 0

	for vehicleID, vehicleRecords := range vehicleGroups {
		if len(vehicleRecords) < 2 {
			continue // Need at least 2 points for distance calculation
		}

		// Sort by timestamp
		sort.Slice(vehicleRecords, func(i, j int) bool {
			timeI, _ := time.Parse("2006-01-02 15:04:05", vehicleRecords[i].Timestamp)
			timeJ, _ := time.Parse("2006-01-02 15:04:05", vehicleRecords[j].Timestamp)
			return timeI.Before(timeJ)
		})

		// Calculate distance and speed for this vehicle
		vehicleDistance := 0.0
		vehicleStops := 0
		lastLat, lastLng := vehicleRecords[0].Lat, vehicleRecords[0].Lng
		lastTime, _ := time.Parse("2006-01-02 15:04:05", vehicleRecords[0].Timestamp)

		// Track dwell periods for stop detection
		dwellStart := lastTime
		inDwell := false

		for i := 1; i < len(vehicleRecords); i++ {
			record := vehicleRecords[i]
			currentTime, _ := time.Parse("2006-01-02 15:04:05", record.Timestamp)

			// Calculate distance from previous point
			segmentDistance := calculateDistanceHaversine(lastLat, lastLng, record.Lat, record.Lng)
			vehicleDistance += segmentDistance

			// Update bearing histogram
			direction := getBearingDirection(float64(record.Bearing))
			analytics.BearingHistogram[direction]++

			// Update hotspots
			gridLat := math.Round(record.Lat/params.Grid) * params.Grid
			gridLng := math.Round(record.Lng/params.Grid) * params.Grid
			gridKey := fmt.Sprintf("%.6f,%.6f", gridLat, gridLng)

			if hotspotMap[gridKey] == nil {
				hotspotMap[gridKey] = make(map[string]bool)
			}
			hotspotMap[gridKey][vehicleID] = true

			// Stop detection logic
			if segmentDistance <= params.StopMaxMoveM {
				// Vehicle is moving slowly or stopped
				if !inDwell {
					// Start new dwell period
					dwellStart = currentTime
					inDwell = true
				}
			} else {
				// Vehicle is moving significantly
				if inDwell {
					// Check if dwell period was long enough to count as a stop
					dwellDuration := currentTime.Sub(dwellStart).Seconds()
					if dwellDuration >= float64(params.StopMinSec) {
						vehicleStops++
						totalStops++
					}
					inDwell = false
				}
			}

			lastLat, lastLng = record.Lat, record.Lng
			lastTime = currentTime
		}

		// Check final dwell period
		if inDwell {
			dwellDuration := lastTime.Sub(dwellStart).Seconds()
			if dwellDuration >= float64(params.StopMinSec) {
				vehicleStops++
				totalStops++
			}
		}

		// Calculate average speed
		firstTime, _ := time.Parse("2006-01-02 15:04:05", vehicleRecords[0].Timestamp)
		timeSpan := lastTime.Sub(firstTime).Hours()
		avgSpeed := 0.0
		if timeSpan > 0 {
			avgSpeed = vehicleDistance / 1000.0 / timeSpan // Convert to km/h
		}

		// Add to totals
		totalDistance += vehicleDistance
		totalTime += timeSpan

		// Create vehicle summary
		vehicleSummary := VehicleSummary{
			VehicleID:   vehicleID,
			Points:      len(vehicleRecords),
			DistanceKM:  vehicleDistance / 1000.0,
			AvgSpeedKMH: avgSpeed,
			Stops:       vehicleStops,
		}
		vehicleSummaries = append(vehicleSummaries, vehicleSummary)
	}

	// Convert hotspots map to slice
	hotspots := []Hotspot{}
	for gridKey, vehicleSet := range hotspotMap {
		var lat, lng float64
		fmt.Sscanf(gridKey, "%f,%f", &lat, &lng)

		hotspots = append(hotspots, Hotspot{
			GridLat:  lat,
			GridLng:  lng,
			Vehicles: len(vehicleSet),
		})
	}

	// Sort hotspots by vehicle count (descending)
	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].Vehicles > hotspots[j].Vehicles
	})

	// Limit hotspots to top 500
	if len(hotspots) > 500 {
		hotspots = hotspots[:500]
	}

	// Fill summary
	analytics.Summary.RecordCount = len(records)
	analytics.Summary.VehicleCount = len(vehicleGroups)
	analytics.Summary.TotalDistanceKM = totalDistance / 1000.0
	analytics.Summary.StopCount = totalStops

	if totalTime > 0 {
		analytics.Summary.AvgSpeedKMH = (totalDistance / 1000.0) / totalTime
	}

	analytics.Hotspots = hotspots
	analytics.Vehicles = vehicleSummaries

	return analytics, nil
}
