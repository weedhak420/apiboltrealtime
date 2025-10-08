package main

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
)

// StmtPool manages prepared statements for better performance
type StmtPool struct {
	insertStmt  *sql.Stmt
	selectStmt  *sql.Stmt
	deleteStmt  *sql.Stmt
	updateStmt  *sql.Stmt
	mutex       sync.RWMutex
	initialized bool
}

// NewStmtPool creates a new statement pool
func NewStmtPool() *StmtPool {
	return &StmtPool{
		initialized: false,
	}
}

// Initialize prepares all statements
func (sp *StmtPool) Initialize(db *sql.DB) error {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	if sp.initialized {
		return nil
	}

	var err error

	// Prepare insert statement
	sp.insertStmt, err = db.Prepare(`
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
		return fmt.Errorf("failed to prepare insert statement: %v", err)
	}

	// Prepare select statement
	sp.selectStmt, err = db.Prepare(`
		SELECT id, lat, lng, bearing, icon_url, category_name, category_id, source_location, timestamp, distance
		FROM vehicle_cache 
		WHERE created_at > DATE_SUB(NOW(), INTERVAL ? MINUTE)
		ORDER BY timestamp DESC
		LIMIT ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare select statement: %v", err)
	}

	// Prepare delete statement
	sp.deleteStmt, err = db.Prepare(`
		DELETE FROM vehicle_cache 
		WHERE created_at < DATE_SUB(NOW(), INTERVAL ? MINUTE)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare delete statement: %v", err)
	}

	// Prepare update statement
	sp.updateStmt, err = db.Prepare(`
		UPDATE vehicle_cache 
		SET lat = ?, lng = ?, bearing = ?, icon_url = ?, category_name = ?, 
		    category_id = ?, source_location = ?, timestamp = ?, distance = ?, created_at = NOW()
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare update statement: %v", err)
	}

	sp.initialized = true
	log.Println("✅ Database statement pool initialized")
	return nil
}

// InsertVehicle inserts a vehicle using prepared statement
func (sp *StmtPool) InsertVehicle(vehicle Vehicle) error {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	if !sp.initialized {
		return fmt.Errorf("statement pool not initialized")
	}

	_, err := sp.insertStmt.Exec(
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
	return err
}

// InsertVehiclesBatch inserts multiple vehicles in a batch
func (sp *StmtPool) InsertVehiclesBatch(vehicles []Vehicle) error {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	if !sp.initialized {
		return fmt.Errorf("statement pool not initialized")
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Use prepared statement in transaction
	stmt := tx.Stmt(sp.insertStmt)
	defer stmt.Close()

	for _, vehicle := range vehicles {
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
			return fmt.Errorf("failed to insert vehicle %s: %v", vehicle.ID, err)
		}
	}

	return tx.Commit()
}

// SelectVehicles selects vehicles using prepared statement
func (sp *StmtPool) SelectVehicles(minutes int, limit int) (*sql.Rows, error) {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	if !sp.initialized {
		return nil, fmt.Errorf("statement pool not initialized")
	}

	return sp.selectStmt.Query(minutes, limit)
}

// DeleteOldVehicles deletes old vehicles using prepared statement
func (sp *StmtPool) DeleteOldVehicles(minutes int) (sql.Result, error) {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	if !sp.initialized {
		return nil, fmt.Errorf("statement pool not initialized")
	}

	return sp.deleteStmt.Exec(minutes)
}

// UpdateVehicle updates a vehicle using prepared statement
func (sp *StmtPool) UpdateVehicle(vehicle Vehicle) error {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	if !sp.initialized {
		return fmt.Errorf("statement pool not initialized")
	}

	_, err := sp.updateStmt.Exec(
		vehicle.Lat,
		vehicle.Lng,
		vehicle.Bearing,
		vehicle.IconURL,
		vehicle.CategoryName,
		vehicle.CategoryID,
		vehicle.SourceLocation,
		vehicle.Timestamp,
		vehicle.Distance,
		vehicle.ID,
	)
	return err
}

// Close closes all prepared statements
func (sp *StmtPool) Close() error {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	var err error

	if sp.insertStmt != nil {
		if e := sp.insertStmt.Close(); e != nil {
			err = e
		}
	}

	if sp.selectStmt != nil {
		if e := sp.selectStmt.Close(); e != nil {
			err = e
		}
	}

	if sp.deleteStmt != nil {
		if e := sp.deleteStmt.Close(); e != nil {
			err = e
		}
	}

	if sp.updateStmt != nil {
		if e := sp.updateStmt.Close(); e != nil {
			err = e
		}
	}

	sp.initialized = false
	return err
}

// Global statement pool
var globalStmtPool *StmtPool

// InitializeStmtPool initializes the global statement pool
func InitializeStmtPool() error {
	globalStmtPool = NewStmtPool()
	return globalStmtPool.Initialize(db)
}

// GetStmtPool returns the global statement pool
func GetStmtPool() *StmtPool {
	return globalStmtPool
}
