package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Database configuration
const (
	DBHost     = "localhost"
	DBPort     = "3306"
	DBUser     = "root"
	DBPassword = ""
	DBName     = "bolt_tracker"
)

// DatabaseStats represents database statistics
type DatabaseStats struct {
	TotalVehicles   int       `json:"total_vehicles"`
	CacheSize       int       `json:"cache_size"`
	LastUpdate      time.Time `json:"last_update"`
	AverageResponse float64   `json:"average_response_ms"`
	ErrorRate       float64   `json:"error_rate"`
	Connections     int       `json:"active_connections"`
	QueryCount      int64     `json:"query_count"`
	CacheHitRate    float64   `json:"cache_hit_rate"`
}

// Initialize database
func initDatabase() error {
	var err error

	// Build DSN dynamically using configuration constants
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		DBUser, DBPassword, DBHost, DBPort, DBName)

	log.Printf("🔗 Connecting to MySQL: %s@%s:%s/%s", DBUser, DBHost, DBPort, DBName)

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("❌ Failed to open database connection: %v", err)
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Test connection
	if err = db.Ping(); err != nil {
		log.Printf("❌ Failed to ping MySQL server: %v", err)
		log.Printf("💡 Make sure MySQL is running and credentials are correct")
		return fmt.Errorf("failed to connect to MySQL: %v", err)
	}

	log.Printf("✅ Successfully connected to MySQL database")

	// Create database if not exists
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS bolt_tracker CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	if err != nil {
		log.Printf("❌ Failed to create database: %v", err)
		return fmt.Errorf("failed to create database: %v", err)
	}
	log.Printf("✅ Database 'bolt_tracker' created/verified")

	// Use the database
	_, err = db.Exec("USE bolt_tracker")
	if err != nil {
		log.Printf("❌ Failed to use database: %v", err)
		return fmt.Errorf("failed to use database: %v", err)
	}

	// Create tables one by one
	tables := []string{
		`CREATE TABLE IF NOT EXISTS vehicle_cache (
			id VARCHAR(255) PRIMARY KEY,
			lat DECIMAL(10, 8),
			lng DECIMAL(11, 8),
			bearing DECIMAL(5, 2),
			icon_url TEXT,
			category_name VARCHAR(255),
			category_id VARCHAR(255),
			source_location VARCHAR(255),
			timestamp DATETIME,
			distance DECIMAL(10, 2),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS vehicle_history (
			history_id BIGINT AUTO_INCREMENT PRIMARY KEY,
			vehicle_id VARCHAR(255),
			lat DOUBLE,
			lng DOUBLE,
			bearing INT,
			category_name VARCHAR(255),
			timestamp DATETIME,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_vehicle_id (vehicle_id),
			INDEX idx_timestamp (timestamp),
			INDEX idx_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS location_cache (
			location_name VARCHAR(255) PRIMARY KEY,
			lat DECIMAL(10, 8),
			lng DECIMAL(11, 8),
			vehicle_count INT,
			last_updated DATETIME,
			success BOOLEAN,
			error TEXT
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS performance_stats (
			id INT AUTO_INCREMENT PRIMARY KEY,
			metric_name VARCHAR(100),
			metric_value DECIMAL(10, 4),
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_metric_time (metric_name, timestamp)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	for i, tableSQL := range tables {
		_, err = db.Exec(tableSQL)
		if err != nil {
			log.Printf("❌ Failed to create table %d: %v", i+1, err)
			return fmt.Errorf("failed to create table %d: %v", i+1, err)
		}
	}
	log.Printf("✅ Database tables created/verified successfully")

	// Create indexes with proper existence checking
	indexes := []struct {
		name   string
		table  string
		column string
		sql    string
	}{
		{
			name:   "idx_vehicle_timestamp",
			table:  "vehicle_cache",
			column: "timestamp",
			sql:    "CREATE INDEX idx_vehicle_timestamp ON vehicle_cache(timestamp)",
		},
		{
			name:   "idx_vehicle_created_at",
			table:  "vehicle_cache",
			column: "created_at",
			sql:    "CREATE INDEX idx_vehicle_created_at ON vehicle_cache(created_at)",
		},
		{
			name:   "idx_vehicle_location",
			table:  "vehicle_cache",
			column: "source_location",
			sql:    "CREATE INDEX idx_vehicle_location ON vehicle_cache(source_location)",
		},
		{
			name:   "idx_vehicle_category",
			table:  "vehicle_cache",
			column: "category_name",
			sql:    "CREATE INDEX idx_vehicle_category ON vehicle_cache(category_name)",
		},
		{
			name:   "idx_location_updated",
			table:  "location_cache",
			column: "last_updated",
			sql:    "CREATE INDEX idx_location_updated ON location_cache(last_updated)",
		},
		// Analytics performance indexes for vehicle_history
		{
			name:   "idx_vehicle_history_time",
			table:  "vehicle_history",
			column: "timestamp",
			sql:    "CREATE INDEX idx_vehicle_history_time ON vehicle_history(timestamp)",
		},
		{
			name:   "idx_vehicle_history_vehicle_time",
			table:  "vehicle_history",
			column: "vehicle_id,timestamp",
			sql:    "CREATE INDEX idx_vehicle_history_vehicle_time ON vehicle_history(vehicle_id, timestamp)",
		},
		{
			name:   "idx_vehicle_history_latlng",
			table:  "vehicle_history",
			column: "lat,lng",
			sql:    "CREATE INDEX idx_vehicle_history_latlng ON vehicle_history(lat, lng)",
		},
		{
			name:   "idx_vehicle_history_category",
			table:  "vehicle_history",
			column: "category_name",
			sql:    "CREATE INDEX idx_vehicle_history_category ON vehicle_history(category_name)",
		},
		{
			name:   "idx_vehicle_history_analytics",
			table:  "vehicle_history",
			column: "timestamp,lat,lng,category_name",
			sql:    "CREATE INDEX idx_vehicle_history_analytics ON vehicle_history(timestamp, lat, lng, category_name)",
		},
	}

	for _, idx := range indexes {
		// Check if index already exists
		var count int
		checkSQL := `
			SELECT COUNT(*) FROM information_schema.statistics 
			WHERE table_schema = DATABASE() 
			AND table_name = ? 
			AND index_name = ?
		`
		err = db.QueryRow(checkSQL, idx.table, idx.name).Scan(&count)
		if err != nil {
			log.Printf("⚠️ Warning: Failed to check index existence for %s: %v", idx.name, err)
			continue
		}

		if count > 0 {
			log.Printf("ℹ️ Index '%s' already exists, skipping", idx.name)
		} else {
			_, err = db.Exec(idx.sql)
			if err != nil {
				log.Printf("❌ Failed to create index '%s': %v", idx.name, err)
			} else {
				log.Printf("✅ Index '%s' created successfully", idx.name)
			}
		}
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("✅ Database initialized successfully with connection pooling")
	return nil
}

// Cache vehicles to database with batch processing
func cacheVehicles(vehicles []Vehicle) error {
	if db == nil {
		log.Printf("❌ Database not initialized - skipping vehicle cache")
		return fmt.Errorf("database not initialized")
	}

	if len(vehicles) == 0 {
		log.Printf("📦 No vehicles to cache")
		return nil
	}

	log.Printf("💾 Caching %d vehicles to database...", len(vehicles))

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Printf("❌ Failed to begin database transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Clear old data (older than 5 minutes) - batch delete
	result, err := tx.Exec("DELETE FROM vehicle_cache WHERE created_at < DATE_SUB(NOW(), INTERVAL 5 MINUTE)")
	if err != nil {
		log.Printf("❌ Failed to clear old data: %v", err)
		return fmt.Errorf("failed to clear old data: %v", err)
	}

	// Log how many old records were deleted
	if rowsAffected, err := result.RowsAffected(); err == nil {
		log.Printf("🗑️ Cleared %d old vehicle records", rowsAffected)
	}

	// Batch insert with prepared statement
	stmt, err := tx.Prepare(`
		INSERT INTO vehicle_cache 
		(id, lat, lng, bearing, icon_url, category_name, category_id, source_location, timestamp, distance, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE
		lat = VALUES(lat),
		lng = VALUES(lng),
		bearing = VALUES(bearing),
		icon_url = VALUES(icon_url),
		category_name = VALUES(category_name),
		category_id = VALUES(category_id),
		source_location = VALUES(source_location),
		timestamp = VALUES(timestamp),
		distance = VALUES(distance),
		created_at = NOW()
	`)
	if err != nil {
		log.Printf("❌ Failed to prepare insert statement: %v", err)
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// Batch process vehicles
	batchSize := 100
	successCount := 0
	errorCount := 0

	for i := 0; i < len(vehicles); i += batchSize {
		end := i + batchSize
		if end > len(vehicles) {
			end = len(vehicles)
		}

		for _, vehicle := range vehicles[i:end] {
			_, err = stmt.Exec(
				vehicle.ID,
				vehicle.Lat,
				vehicle.Lng,
				vehicle.Bearing,
				vehicle.IconURL,
				vehicle.CategoryName,
				vehicle.CategoryID,
				vehicle.SourceLocation,
				vehicle.Timestamp,
				vehicle.Distance,
			)
			if err != nil {
				log.Printf("⚠️ Failed to cache vehicle %s: %v", vehicle.ID, err)
				errorCount++
			} else {
				successCount++
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Printf("❌ Failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("✅ Successfully cached %d vehicles to database (errors: %d)", successCount, errorCount)
	return nil
}

// Get cached vehicles with optimized query
func getCachedVehicles() ([]Vehicle, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Optimized query with LIMIT and recent data only
	rows, err := db.Query(`
		SELECT id, lat, lng, bearing, icon_url, category_name, category_id, source_location, timestamp, distance
		FROM vehicle_cache 
		WHERE created_at > DATE_SUB(NOW(), INTERVAL 2 MINUTE)
		ORDER BY timestamp DESC
		LIMIT 1000
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query vehicles: %v", err)
	}
	defer rows.Close()

	var vehicles []Vehicle
	for rows.Next() {
		var v Vehicle
		err := rows.Scan(
			&v.ID,
			&v.Lat,
			&v.Lng,
			&v.Bearing,
			&v.IconURL,
			&v.CategoryName,
			&v.CategoryID,
			&v.SourceLocation,
			&v.Timestamp,
			&v.Distance,
		)
		if err != nil {
			log.Printf("⚠️ Failed to scan vehicle: %v", err)
			continue
		}
		vehicles = append(vehicles, v)
	}

	return vehicles, nil
}

// Get database statistics
func getDatabaseStats() (DatabaseStats, error) {
	if db == nil {
		return DatabaseStats{}, fmt.Errorf("database not initialized")
	}

	stats := DatabaseStats{}

	// Get total vehicles
	err := db.QueryRow("SELECT COUNT(*) FROM vehicle_cache").Scan(&stats.TotalVehicles)
	if err != nil {
		return stats, fmt.Errorf("failed to get total vehicles: %v", err)
	}

	// Get cache size
	err = db.QueryRow("SELECT COUNT(*) FROM vehicle_cache WHERE created_at > DATE_SUB(NOW(), INTERVAL 5 MINUTE)").Scan(&stats.CacheSize)
	if err != nil {
		return stats, fmt.Errorf("failed to get cache size: %v", err)
	}

	// Get last update
	err = db.QueryRow("SELECT MAX(created_at) FROM vehicle_cache").Scan(&stats.LastUpdate)
	if err != nil {
		stats.LastUpdate = time.Now()
	}

	// Get database connections
	err = db.QueryRow("SHOW STATUS LIKE 'Threads_connected'").Scan(&stats.Connections)
	if err != nil {
		stats.Connections = 0
	}

	// Calculate cache hit rate (simplified)
	stats.CacheHitRate = 0.85 // Placeholder - implement proper calculation

	return stats, nil
}

// Record performance metric
func recordPerformanceMetric(metricName string, value float64) error {
	if db == nil {
		return nil // Skip if database not available
	}

	_, err := db.Exec("INSERT INTO performance_stats (metric_name, metric_value) VALUES (?, ?)", metricName, value)
	return err
}

// Clean old performance data
func cleanOldPerformanceData() error {
	if db == nil {
		return nil
	}

	_, err := db.Exec("DELETE FROM performance_stats WHERE timestamp < DATE_SUB(NOW(), INTERVAL 24 HOUR)")
	return err
}

// insertVehicleHistory inserts vehicle data into history table (insert-only)
func insertVehicleHistory(vehicles []Vehicle) error {
	if db == nil {
		log.Printf("❌ Database not initialized - skipping vehicle history")
		return fmt.Errorf("database not initialized")
	}

	if len(vehicles) == 0 {
		log.Printf("📦 No vehicles to insert into history")
		return nil
	}

	log.Printf("📝 Inserting %d vehicles into history...", len(vehicles))

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Printf("❌ Failed to begin database transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Batch insert with prepared statement
	stmt, err := tx.Prepare(`
		INSERT INTO vehicle_history 
		(vehicle_id, lat, lng, bearing, category_name, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Printf("❌ Failed to prepare insert statement: %v", err)
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// Batch process vehicles
	batchSize := 100
	successCount := 0
	errorCount := 0

	for i := 0; i < len(vehicles); i += batchSize {
		end := i + batchSize
		if end > len(vehicles) {
			end = len(vehicles)
		}

		for _, vehicle := range vehicles[i:end] {
			_, err = stmt.Exec(
				vehicle.ID,
				vehicle.Lat,
				vehicle.Lng,
				int(vehicle.Bearing),
				vehicle.CategoryName,
				vehicle.Timestamp,
			)
			if err != nil {
				log.Printf("⚠️ Failed to insert vehicle history %s: %v", vehicle.ID, err)
				errorCount++
			} else {
				successCount++
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Printf("❌ Failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("✅ Successfully inserted %d vehicles into history (errors: %d)", successCount, errorCount)
	return nil
}

// getVehicleHistory retrieves vehicle history with optional filtering
func getVehicleHistory(limit int, vehicleID string) ([]HistoryRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var query string
	var args []interface{}

	if vehicleID != "" {
		query = `
			SELECT history_id, vehicle_id, lat, lng, bearing, category_name, timestamp, created_at
			FROM vehicle_history 
			WHERE vehicle_id = ?
			ORDER BY timestamp DESC
			LIMIT ?
		`
		args = []interface{}{vehicleID, limit}
	} else {
		query = `
			SELECT history_id, vehicle_id, lat, lng, bearing, category_name, timestamp, created_at
			FROM vehicle_history 
			ORDER BY timestamp DESC
			LIMIT ?
		`
		args = []interface{}{limit}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query vehicle history: %v", err)
	}
	defer rows.Close()

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

	return records, nil
}

// getVehicleHistoryWithFilters retrieves vehicle history with enhanced filtering for the new API
func getVehicleHistoryWithFilters(vehicleID, startTime, endTime string, limit int) ([]VehicleHistory, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Build query with optional filters
	query := `SELECT vehicle_id, lat, lng, bearing, timestamp, category_name 
			  FROM vehicle_history WHERE 1=1`
	args := []interface{}{}

	if vehicleID != "" {
		query += " AND vehicle_id = ?"
		args = append(args, vehicleID)
	}
	if startTime != "" {
		query += " AND timestamp >= ?"
		args = append(args, startTime)
	}
	if endTime != "" {
		query += " AND timestamp <= ?"
		args = append(args, endTime)
	}

	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query vehicle history: %v", err)
	}
	defer rows.Close()

	var history []VehicleHistory
	for rows.Next() {
		var vh VehicleHistory
		var timestamp time.Time

		err := rows.Scan(&vh.ID, &vh.Lat, &vh.Lng, &vh.Bearing, &timestamp, &vh.CategoryName)
		if err != nil {
			log.Printf("⚠️ Failed to scan vehicle history row: %v", err)
			continue
		}

		vh.Timestamp = timestamp
		history = append(history, vh)
	}

	return history, nil
}

// getHeatmapData retrieves heatmap data for analytics
func getHeatmapData(limit int) ([]Hotspot, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Count unique vehicles per grid cell instead of total points
	query := `
		SELECT ROUND(lat,3) AS grid_lat, ROUND(lng,3) AS grid_lng, COUNT(DISTINCT vehicle_id) AS vehicles
		FROM vehicle_history
		WHERE timestamp > DATE_SUB(NOW(), INTERVAL 24 HOUR)
		GROUP BY grid_lat, grid_lng
		ORDER BY vehicles DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query heatmap data: %v", err)
	}
	defer rows.Close()

	var hotspots []Hotspot
	for rows.Next() {
		var hotspot Hotspot
		err := rows.Scan(&hotspot.GridLat, &hotspot.GridLng, &hotspot.Vehicles)
		if err != nil {
			log.Printf("⚠️ Failed to scan heatmap data: %v", err)
			continue
		}
		hotspots = append(hotspots, hotspot)
	}

	return hotspots, nil
}

// getTrendData retrieves trend data for analytics with smoothing
func getTrendData(interval string) ([]TrendPoint, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var query string
	var limit int

	switch interval {
	case "hour":
		query = `
			SELECT DATE_FORMAT(timestamp, '%Y-%m-%d %H:00:00') AS hour, COUNT(DISTINCT vehicle_id) AS vehicles
			FROM vehicle_history
			WHERE timestamp > DATE_SUB(NOW(), INTERVAL 24 HOUR)
			GROUP BY hour
			ORDER BY hour ASC
			LIMIT ?
		`
		limit = 24
	case "day":
		query = `
			SELECT DATE_FORMAT(timestamp, '%Y-%m-%d 00:00:00') AS day, COUNT(DISTINCT vehicle_id) AS vehicles
			FROM vehicle_history
			WHERE timestamp > DATE_SUB(NOW(), INTERVAL 7 DAY)
			GROUP BY day
			ORDER BY day ASC
			LIMIT ?
		`
		limit = 7
	default:
		return nil, fmt.Errorf("invalid interval: %s (supported: hour, day)", interval)
	}

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query trend data: %v", err)
	}
	defer rows.Close()

	var trends []TrendPoint
	for rows.Next() {
		var trend TrendPoint
		err := rows.Scan(&trend.Time, &trend.Vehicles)
		if err != nil {
			log.Printf("⚠️ Failed to scan trend data: %v", err)
			continue
		}
		trends = append(trends, trend)
	}

	// Apply exponential moving average smoothing (alpha = 0.3)
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
