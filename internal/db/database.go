package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database wraps SQLite connection
type Database struct {
	db *sql.DB
}

// TestRecord represents a test in the database
type TestRecord struct {
	ID          string    `json:"id"`
	GameURL     string    `json:"gameUrl"`
	Status      string    `json:"status"`
	Score       int       `json:"score"`
	Duration    int       `json:"duration"`
	ReportID    string    `json:"reportId"`
	ReportData  string    `json:"reportData"` // JSON string
	CreatedAt   time.Time `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// New creates a new database connection and initializes the schema
func New(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize schema
	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &Database{db: db}, nil
}

// initSchema creates the necessary tables
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tests (
		id TEXT PRIMARY KEY,
		game_url TEXT NOT NULL,
		status TEXT NOT NULL,
		score INTEGER DEFAULT 0,
		duration INTEGER DEFAULT 0,
		report_id TEXT,
		report_data TEXT,
		created_at TIMESTAMP NOT NULL,
		completed_at TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_tests_created_at ON tests(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_tests_status ON tests(status);
	CREATE INDEX IF NOT EXISTS idx_tests_game_url ON tests(game_url);
	`

	_, err := db.Exec(schema)
	return err
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// CreateTest inserts a new test record
func (d *Database) CreateTest(id, gameURL, status string) error {
	query := `
		INSERT INTO tests (id, game_url, status, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := d.db.Exec(query, id, gameURL, status, time.Now())
	return err
}

// UpdateTestStatus updates the status of a test
func (d *Database) UpdateTestStatus(id, status string) error {
	query := `UPDATE tests SET status = ? WHERE id = ?`
	_, err := d.db.Exec(query, status, id)
	return err
}

// CompleteTest marks a test as complete with final data
func (d *Database) CompleteTest(id, status string, score, duration int, reportID string, reportData interface{}) error {
	// Convert report data to JSON
	reportJSON, err := json.Marshal(reportData)
	if err != nil {
		return fmt.Errorf("failed to marshal report data: %w", err)
	}

	query := `
		UPDATE tests
		SET status = ?, score = ?, duration = ?, report_id = ?, report_data = ?, completed_at = ?
		WHERE id = ?
	`
	now := time.Now()
	_, err = d.db.Exec(query, status, score, duration, reportID, string(reportJSON), now, id)
	return err
}

// GetTest retrieves a test by ID
func (d *Database) GetTest(id string) (*TestRecord, error) {
	query := `
		SELECT id, game_url, status, score, duration, report_id, report_data, created_at, completed_at
		FROM tests
		WHERE id = ?
	`

	var test TestRecord
	var reportData sql.NullString
	var completedAt sql.NullTime

	err := d.db.QueryRow(query, id).Scan(
		&test.ID,
		&test.GameURL,
		&test.Status,
		&test.Score,
		&test.Duration,
		&test.ReportID,
		&reportData,
		&test.CreatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if reportData.Valid {
		test.ReportData = reportData.String
	}
	if completedAt.Valid {
		test.CompletedAt = &completedAt.Time
	}

	return &test, nil
}

// GetTestByReportID retrieves a test by report ID
func (d *Database) GetTestByReportID(reportID string) (*TestRecord, error) {
	query := `
		SELECT id, game_url, status, score, duration, report_id, report_data, created_at, completed_at
		FROM tests
		WHERE report_id = ?
	`

	var test TestRecord
	var reportData sql.NullString
	var completedAt sql.NullTime

	err := d.db.QueryRow(query, reportID).Scan(
		&test.ID,
		&test.GameURL,
		&test.Status,
		&test.Score,
		&test.Duration,
		&test.ReportID,
		&reportData,
		&test.CreatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if reportData.Valid {
		test.ReportData = reportData.String
	}
	if completedAt.Valid {
		test.CompletedAt = &completedAt.Time
	}

	return &test, nil
}

// ListTests retrieves all tests with optional filtering
func (d *Database) ListTests(status string, limit, offset int) ([]TestRecord, error) {
	query := `
		SELECT id, game_url, status, score, duration, report_id, report_data, created_at, completed_at
		FROM tests
		WHERE 1=1
	`
	args := []interface{}{}

	if status != "" && status != "all" {
		query += ` AND status = ?`
		args = append(args, status)
	}

	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []TestRecord
	for rows.Next() {
		var test TestRecord
		var reportData sql.NullString
		var completedAt sql.NullTime

		err := rows.Scan(
			&test.ID,
			&test.GameURL,
			&test.Status,
			&test.Score,
			&test.Duration,
			&test.ReportID,
			&reportData,
			&test.CreatedAt,
			&completedAt,
		)
		if err != nil {
			return nil, err
		}

		if reportData.Valid {
			test.ReportData = reportData.String
		}
		if completedAt.Valid {
			test.CompletedAt = &completedAt.Time
		}

		tests = append(tests, test)
	}

	return tests, rows.Err()
}

// CountTests returns the total number of tests
func (d *Database) CountTests(status string) (int, error) {
	query := `SELECT COUNT(*) FROM tests WHERE 1=1`
	args := []interface{}{}

	if status != "" && status != "all" {
		query += ` AND status = ?`
		args = append(args, status)
	}

	var count int
	err := d.db.QueryRow(query, args...).Scan(&count)
	return count, err
}
