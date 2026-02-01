package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

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
		medical_checks TEXT,
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
	INSERT INTO recordings (file_name, upload_time, size_kb, transcript, dialogue, medical_checks, medical_record, related_command) 
	VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`
	_, err := DB.Exec(stmt, r.FileName, r.UploadTime, r.SizeKB, r.Transcript, r.Dialogue, r.MedicalChecks, r.MedicalRecord, r.RelatedCommand)
	if err != nil {
		return fmt.Errorf("failed to insert recording: %w", err)
	}
	log.Printf("Successfully logged new recording: %s", r.FileName)
	return nil
}

func GetRecentRecordings(limit int) ([]models.Recording, error) {
	rows, err := DB.Query("SELECT id, file_name, upload_time, size_kb, transcript, dialogue, medical_checks, medical_record, related_command FROM recordings ORDER BY upload_time DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recordings []models.Recording
	for rows.Next() {
		var r models.Recording
		if err := rows.Scan(&r.ID, &r.FileName, &r.UploadTime, &r.SizeKB, &r.Transcript, &r.Dialogue, &r.MedicalChecks, &r.MedicalRecord, &r.RelatedCommand); err != nil {
			return nil, err
		}
		recordings = append(recordings, r)
	}
	return recordings, nil
}

// UpdateRecordingMedicalRecord 更新指定录音的病历记录
func UpdateRecordingMedicalRecord(fileName string, medicalRecord string) error {
	const stmt = `
	UPDATE recordings 
	SET medical_record = ?, related_command = ?
	WHERE file_name = ?;
	`

	// 更新相关命令为包含Baichuan处理完成的信息
	relatedCommand := "(ASR和Baichuan处理完成)"

	_, err := DB.Exec(stmt, medicalRecord, relatedCommand, fileName)
	if err != nil {
		return fmt.Errorf("failed to update recording medical record: %w", err)
	}

	log.Printf("Successfully updated medical record for recording: %s", fileName)
	return nil
}

func UpdateRecordingMedicalChecks(fileName string, medicalChecks string) error {
	const stmt = `
	UPDATE recordings
	SET medical_checks = ?, related_command = ?
	WHERE file_name = ?;
	`

	relatedCommand := "(Medical Checks上传完成)"

	_, err := DB.Exec(stmt, medicalChecks, relatedCommand, fileName)
	if err != nil {
		return fmt.Errorf("failed to update recording medical checks: %w", err)
	}

	log.Printf("Successfully updated medical checks for recording: %s", fileName)
	return nil
}

type MobileRecord struct {
	ID              int       `json:"id"`
	DiagnosisRecord string    `json:"diagnosis_record"`
	AudioFile       string    `json:"audio_file"`
	AudioText       string    `json:"audio_text"`
	HISRecord       string    `json:"his_record"`
	CreatedAt       time.Time `json:"created_at"`
}

func GetMobileRecords() ([]MobileRecord, error) {
	rows, err := DB.Query("SELECT id, medical_checks, file_name, dialogue, medical_record, upload_time FROM recordings ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []MobileRecord
	for rows.Next() {
		var rec MobileRecord
		err := rows.Scan(&rec.ID, &rec.DiagnosisRecord, &rec.AudioFile, &rec.AudioText, &rec.HISRecord, &rec.CreatedAt)
		if err != nil {
			log.Printf("Row scan error: %v\n", err)
			continue
		}
		records = append(records, rec)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

func InsertMobileRecording(fileName string) (int64, error) {
	result, err := DB.Exec(
		"INSERT INTO recordings (file_name, upload_time, size_kb, transcript, dialogue, medical_checks, medical_record, related_command) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		fileName, time.Now(), 0, "", "", "", "", "(Mobile上传)",
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert mobile recording: %w", err)
	}

	id, _ := result.LastInsertId()
	log.Printf("Successfully inserted mobile recording: id=%d, file=%s", id, fileName)
	return id, nil
}

func UpdateMobileDiagnosis(id int, content string) error {
	_, err := DB.Exec("UPDATE recordings SET medical_checks = ? WHERE id = ?", content, id)
	if err != nil {
		return fmt.Errorf("failed to update mobile diagnosis: %w", err)
	}

	log.Printf("Successfully updated mobile diagnosis for record: %d", id)
	return nil
}

// GetMobileRecordByID 根据ID获取移动端记录
func GetMobileRecordByID(id int) (MobileRecord, error) {
	var rec MobileRecord
	err := DB.QueryRow("SELECT id, medical_checks, file_name, transcript, medical_record, upload_time FROM recordings WHERE id = ?", id).Scan(
		&rec.ID, &rec.DiagnosisRecord, &rec.AudioFile, &rec.AudioText, &rec.HISRecord, &rec.CreatedAt,
	)
	if err != nil {
		return rec, fmt.Errorf("failed to get mobile record by id: %w", err)
	}
	return rec, nil
}

func UpdateMobileAudioText(id int, audioText string) error {
	_, err := DB.Exec("UPDATE recordings SET dialogue = ? WHERE id = ?", audioText, id)
	if err != nil {
		return fmt.Errorf("failed to update mobile audio text: %w", err)
	}

	log.Printf("Successfully updated mobile audio text for record: %d", id)
	return nil
}

// UpdateMobileHISRecord 更新移动端记录的HIS诊疗记录
func UpdateMobileHISRecord(id int, hisRecord string) error {
	_, err := DB.Exec("UPDATE recordings SET medical_record = ? WHERE id = ?", hisRecord, id)
	if err != nil {
		return fmt.Errorf("failed to update mobile HIS record: %w", err)
	}

	log.Printf("Successfully updated mobile HIS record for record: %d", id)
	return nil
}
