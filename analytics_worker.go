package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// AnalyticsWorker handles background pre-computation of analytics data
type AnalyticsWorker struct {
	ticker *time.Ticker
	ctx    context.Context
	cancel context.CancelFunc
}

// NewAnalyticsWorker creates a new analytics worker
func NewAnalyticsWorker() *AnalyticsWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &AnalyticsWorker{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the analytics worker
func (w *AnalyticsWorker) Start() {
	w.ticker = time.NewTicker(30 * time.Second)

	log.Println("🚀 Starting analytics background worker...")

	go func() {
		for {
			select {
			case <-w.ticker.C:
				w.precomputeAnalytics()
			case <-w.ctx.Done():
				log.Println("🛑 Analytics worker stopped")
				return
			}
		}
	}()
}

// Stop stops the analytics worker
func (w *AnalyticsWorker) Stop() {
	if w.ticker != nil {
		w.ticker.Stop()
	}
	w.cancel()
}

// precomputeAnalytics runs all analytics pre-computation
func (w *AnalyticsWorker) precomputeAnalytics() {
	log.Println("🔄 Pre-computing analytics data...")

	// Run all pre-computations in parallel
	go w.precomputeHeatmap()
	go w.precomputeTrend()
	go w.precomputeHistorySummary()
}

// precomputeHeatmap pre-computes heatmap data
func (w *AnalyticsWorker) precomputeHeatmap() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ Heatmap pre-computation panic recovered: %v", r)
		}
	}()

	// Get heatmap data from database
	hotspots, err := getHeatmapData(500)
	if err != nil {
		log.Printf("❌ Failed to get heatmap data: %v", err)
		return
	}

	// Cache the result
	result := map[string]interface{}{
		"hotspots":  hotspots,
		"count":     len(hotspots),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	err = cacheHeatmapData("10min", result)
	if err != nil {
		log.Printf("⚠️ Failed to cache heatmap data: %v", err)
	} else {
		log.Printf("✅ Cached heatmap data: %d hotspots", len(hotspots))
	}
}

// precomputeTrend pre-computes trend data
func (w *AnalyticsWorker) precomputeTrend() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ Trend pre-computation panic recovered: %v", r)
		}
	}()

	// Pre-compute for both hour and day intervals
	intervals := []string{"hour", "day"}

	for _, interval := range intervals {
		trends, err := getTrendData(interval)
		if err != nil {
			log.Printf("❌ Failed to get trend data for %s: %v", interval, err)
			continue
		}

		// Cache the result
		result := map[string]interface{}{
			"trends":    trends,
			"count":     len(trends),
			"interval":  interval,
			"smoothing": "ema_0.3",
			"timestamp": time.Now().Format(time.RFC3339),
		}

		err = cacheTrendData(interval, result)
		if err != nil {
			log.Printf("⚠️ Failed to cache trend data for %s: %v", interval, err)
		} else {
			log.Printf("✅ Cached trend data for %s: %d points", interval, len(trends))
		}
	}
}

// precomputeHistorySummary pre-computes common history analytics
func (w *AnalyticsWorker) precomputeHistorySummary() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ History summary pre-computation panic recovered: %v", r)
		}
	}()

	// Pre-compute for common time ranges
	now := time.Now()
	timeRanges := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{"1hour", now.Add(-1 * time.Hour), now},
		{"6hours", now.Add(-6 * time.Hour), now},
		{"24hours", now.Add(-24 * time.Hour), now},
		{"7days", now.Add(-7 * 24 * time.Hour), now},
	}

	for _, tr := range timeRanges {
		// Create parameters for history analytics
		params := HistoryAnalyticsParams{
			Start:        tr.start,
			End:          tr.end,
			VehicleID:    "", // All vehicles
			BBox:         nil,
			Limit:        1000,
			Offset:       0,
			Grid:         0.001, // ~110m grid
			StopMinSec:   120,   // 2 minutes
			StopMaxMoveM: 10.0,  // 10 meters
		}

		// Get analytics data
		analytics, err := getHistoryAnalytics(params)
		if err != nil {
			log.Printf("❌ Failed to get history analytics for %s: %v", tr.name, err)
			continue
		}

		// Cache the result
		result := map[string]interface{}{
			"summary":           analytics.Summary,
			"bearing_histogram": analytics.BearingHistogram,
			"hotspots":          analytics.Hotspots,
			"vehicles":          analytics.Vehicles,
			"time_range":        tr.name,
			"timestamp":         time.Now().Format(time.RFC3339),
		}

		// Create cache key
		startStr := tr.start.Format("2006-01-02T15:04:05Z")
		endStr := tr.end.Format("2006-01-02T15:04:05Z")
		gridStr := fmt.Sprintf("%.3f", params.Grid)

		err = cacheHistoryData("", startStr, endStr, gridStr, result)
		if err != nil {
			log.Printf("⚠️ Failed to cache history data for %s: %v", tr.name, err)
		} else {
			log.Printf("✅ Cached history data for %s: %d vehicles, %d hotspots",
				tr.name, analytics.Summary.VehicleCount, len(analytics.Hotspots))
		}
	}
}

// Global analytics worker instance
var analyticsWorker *AnalyticsWorker

// StartAnalyticsWorker starts the global analytics worker
func StartAnalyticsWorker() {
	if analyticsWorker != nil {
		log.Println("⚠️ Analytics worker already running")
		return
	}

	analyticsWorker = NewAnalyticsWorker()
	analyticsWorker.Start()
	log.Println("✅ Analytics worker started")
}

// StopAnalyticsWorker stops the global analytics worker
func StopAnalyticsWorker() {
	if analyticsWorker != nil {
		analyticsWorker.Stop()
		analyticsWorker = nil
		log.Println("✅ Analytics worker stopped")
	}
}

// Pre-compute optimized heatmap data with grid aggregation
func precomputeOptimizedHeatmap() ([]Hotspot, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Optimized query with grid aggregation
	query := `
		SELECT 
			ROUND(lat, 3) as grid_lat, 
			ROUND(lng, 3) as grid_lng, 
			COUNT(DISTINCT vehicle_id) as vehicles
		FROM vehicle_history
		WHERE timestamp >= NOW() - INTERVAL 10 MINUTE
		GROUP BY grid_lat, grid_lng
		HAVING vehicles > 0
		ORDER BY vehicles DESC
		LIMIT 500
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query optimized heatmap: %v", err)
	}
	defer rows.Close()

	var hotspots []Hotspot
	for rows.Next() {
		var hotspot Hotspot
		err := rows.Scan(&hotspot.GridLat, &hotspot.GridLng, &hotspot.Vehicles)
		if err != nil {
			log.Printf("⚠️ Failed to scan heatmap row: %v", err)
			continue
		}
		hotspots = append(hotspots, hotspot)
	}

	return hotspots, nil
}

// Pre-compute optimized trend data with time bucketing
func precomputeOptimizedTrend(interval string) ([]TrendPoint, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var query string
	var limit int

	switch interval {
	case "hour":
		query = `
			SELECT 
				DATE_FORMAT(timestamp, '%Y-%m-%d %H:00:00') AS time_bucket,
				COUNT(DISTINCT vehicle_id) AS vehicles
			FROM vehicle_history
			WHERE timestamp >= NOW() - INTERVAL 24 HOUR
			GROUP BY time_bucket
			ORDER BY time_bucket ASC
			LIMIT ?
		`
		limit = 24
	case "day":
		query = `
			SELECT 
				DATE_FORMAT(timestamp, '%Y-%m-%d 00:00:00') AS time_bucket,
				COUNT(DISTINCT vehicle_id) AS vehicles
			FROM vehicle_history
			WHERE timestamp >= NOW() - INTERVAL 7 DAY
			GROUP BY time_bucket
			ORDER BY time_bucket ASC
			LIMIT ?
		`
		limit = 7
	default:
		return nil, fmt.Errorf("invalid interval: %s", interval)
	}

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query optimized trend: %v", err)
	}
	defer rows.Close()

	var trends []TrendPoint
	for rows.Next() {
		var trend TrendPoint
		err := rows.Scan(&trend.Time, &trend.Vehicles)
		if err != nil {
			log.Printf("⚠️ Failed to scan trend row: %v", err)
			continue
		}
		trends = append(trends, trend)
	}

	// Apply exponential moving average smoothing
	if len(trends) > 1 {
		alpha := 0.3
		trends[0].Smoothed = float64(trends[0].Vehicles)

		for i := 1; i < len(trends); i++ {
			trends[i].Smoothed = alpha*float64(trends[i].Vehicles) + (1-alpha)*trends[i-1].Smoothed
		}
	} else if len(trends) == 1 {
		trends[0].Smoothed = float64(trends[0].Vehicles)
	}

	return trends, nil
}
