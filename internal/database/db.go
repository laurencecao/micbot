package database

import (
	"database/sql"
	"fmt"
	"log"

	"medishare.io/micbot/internal/config"
	"medishare.io/micbot/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() {
	var err error
	DB, err = sql.Open("sqlite3", config.DBPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	// 创建表
	createTables := `
	CREATE TABLE IF NOT EXISTS agents (
		session_id TEXT PRIMARY KEY,
		status TEXT,
		last_update DATETIME
	);
	CREATE TABLE IF NOT EXISTS recordings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_name TEXT,
		upload_time DATETIME,
		size_kb INTEGER,
		transcript TEXT,
		dialogue TEXT,
		medical_record TEXT,
		related_command TEXT
	);
	`
	_, err = DB.Exec(createTables)
	if err != nil {
		log.Fatalf("Error creating tables: %v", err)
	}
	log.Println("Database initialized successfully.")
}

// --- Agent Status Functions ---

func UpdateAgentStatus(status models.AgentStatus) error {
	const stmt = `
	INSERT INTO agents (session_id, status, last_update) 
	VALUES (?, ?, ?) 
	ON CONFLICT(session_id) DO UPDATE SET 
		status=excluded.status, last_update=excluded.last_update;
	`
	_, err := DB.Exec(stmt, status.SessionID, string(status.Status), status.LastUpdate)
	if err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}
	return nil
}

func GetAllAgentStatuses() ([]models.AgentStatus, error) {
	rows, err := DB.Query("SELECT session_id, status, last_update FROM agents WHERE last_update >= datetime('now', 'localtime', '-1 minute') ORDER BY last_update DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []models.AgentStatus
	for rows.Next() {
		var s models.AgentStatus
		var statusStr string
		if err := rows.Scan(&s.SessionID, &statusStr, &s.LastUpdate); err != nil {
			return nil, err
		}
		s.Status = models.AgentState(statusStr)
		statuses = append(statuses, s)
	}
	return statuses, nil
}

// --- Recording Functions ---

func InsertRecording(r models.Recording) error {
	const stmt = `
	INSERT INTO recordings (file_name, upload_time, size_kb, transcript, dialogue, medical_record, related_command) 
	VALUES (?, ?, ?, ?, ?, ?, ?);
	`
	_, err := DB.Exec(stmt, r.FileName, r.UploadTime, r.SizeKB, r.Transcript, r.Dialogue, r.MedicalRecord, r.RelatedCommand)
	if err != nil {
		return fmt.Errorf("failed to insert recording: %w", err)
	}
	log.Printf("Successfully logged new recording: %s", r.FileName)
	return nil
}

func GetRecentRecordings(limit int) ([]models.Recording, error) {
	rows, err := DB.Query("SELECT file_name, upload_time, size_kb, transcript, dialogue, medical_record, related_command FROM recordings ORDER BY upload_time DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recordings []models.Recording
	for rows.Next() {
		var r models.Recording
		// Note: ID is auto-incremented, no need to read it here
		if err := rows.Scan(&r.FileName, &r.UploadTime, &r.SizeKB, &r.Transcript, &r.Dialogue, &r.MedicalRecord, &r.RelatedCommand); err != nil {
			return nil, err
		}
		recordings = append(recordings, r)
	}
	return recordings, nil
}
