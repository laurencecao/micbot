package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"medishare.io/micbot/internal/config"
	"medishare.io/micbot/internal/database"
	"medishare.io/micbot/internal/models"

	"github.com/nats-io/nats.go"
)

var (
	natsConn *nats.Conn
	tmpl     *template.Template
)

// initServer 初始化数据库和NATS连接
func initServer() {
	database.InitDB()

	var err error
	// 确保 NatsURL 和 DBPath 已经在 internal/config/config.go 中定义
	natsConn, err = nats.Connect(config.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	log.Println("Server connected to NATS.")

	// 编译前端模板
	tmpl = template.Must(template.ParseFiles("web/index.html"))
}

// subscribeAgentStatus 监听 Agent 上报的状态，并更新数据库
// 这是之前丢失的函数之一
func subscribeAgentStatus() {
	natsConn.Subscribe(config.StatusReport, func(m *nats.Msg) {
		var status models.AgentStatus
		if err := json.Unmarshal(m.Data, &status); err != nil {
			log.Printf("Failed to unmarshal agent status: %v", err)
			return
		}

		// 数据库操作：更新 Agent 状态
		if err := database.UpdateAgentStatus(status); err != nil {
			log.Printf("DB error updating agent status: %v", err)
		} else {
			log.Printf("[NATS] Received and updated status for Agent %s: %s", status.SessionID[:4], status.Status)
		}
	})
}

// subscribeUploadRecord 监听 Agent 上传成功后的反馈（用于DB记录）
// 这是之前丢失的函数之一
func subscribeUploadRecord() {
	natsConn.Subscribe(config.CmdUploadRecord, func(m *nats.Msg) {
		var cmd models.CommandMessage
		if err := json.Unmarshal(m.Data, &cmd); err != nil {
			log.Printf("Failed to unmarshal upload command for logging: %v", err)
			return
		}

		// MOCK: Generate metadata for the DB
		newRecord := models.Recording{
			FileName:   cmd.Payload,
			UploadTime: time.Now(),
			SizeKB:     rand.Intn(5000) + 1000, // 1MB to 6MB mock size
			Transcript: "This is a mock transcription of the recorded audio: 'Hello World, NATS is great!'",
		}

		// 写入数据库
		if err := database.InsertRecording(newRecord); err != nil {
			log.Printf("DB error inserting new recording: %v", err)
		}
	})
}

// apiStatusHandler 返回 Agent 状态 JSON
func apiStatusHandler(w http.ResponseWriter, r *http.Request) {
	statuses, err := database.GetAllAgentStatuses()
	if err != nil {
		log.Println("Error fetching agent statuses:", err)
		http.Error(w, "Failed to fetch status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// 注意：这里 Go 会自动将 time.Time 编码为 RFC3339 格式的字符串，前端 JS 可以解析
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		log.Printf("Error encoding status JSON: %v", err)
	}
}

// apiHistoryHandler 返回录音历史 JSON
func apiHistoryHandler(w http.ResponseWriter, r *http.Request) {
	history, err := database.GetRecentRecordings(10)
	if err != nil {
		log.Println("Error fetching recording history:", err)
		http.Error(w, "Failed to fetch history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(history); err != nil {
		log.Printf("Error encoding history JSON: %v", err)
	}
}

// homeHandler 仅用于渲染初始 HTML 结构
func homeHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		AgentStatuses    []models.AgentStatus
		RecordingHistory []models.Recording
	}{
		AgentStatuses:    []models.AgentStatus{},
		RecordingHistory: []models.Recording{},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

// commandHandler 处理前端发送的指令 (Start/Stop)
func commandHandler(w http.ResponseWriter, r *http.Request) {
	cmd := r.URL.Query().Get("action")
	subject := ""

	switch cmd {
	case "start_record":
		subject = config.CmdStartRecord
	case "stop_record":
		subject = config.CmdStopRecord
	default:
		http.Error(w, "Invalid command", http.StatusBadRequest)
		return
	}

	msg := models.CommandMessage{}
	data, _ := json.Marshal(msg)

	resp, err := natsConn.Request(subject, data, 3*time.Second)

	if err != nil {
		log.Printf("NATS Request error for %s: %v", cmd, err)
		http.Error(w, fmt.Sprintf("Command failed or timed out: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Command '%s' sent successfully. Agent Response: %s", cmd, string(resp.Data))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Command %s processed. Agent response: %s", cmd, string(resp.Data))
}

func main() {
	if _, err := os.Stat("web/index.html"); os.IsNotExist(err) {
		log.Fatal("web/index.html not found. Please create the frontend template.")
	}

	initServer()
	defer natsConn.Close()

	// 启动监听器 (现在函数定义已经存在)
	subscribeAgentStatus()
	subscribeUploadRecord()

	// 配置 HTTP 路由
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/api/command", commandHandler)
	http.HandleFunc("/api/status", apiStatusHandler)
	http.HandleFunc("/api/history", apiHistoryHandler)

	port := ":8080"
	log.Printf("Web Server running on http://localhost%s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}
