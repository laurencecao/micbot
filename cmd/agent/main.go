package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"medishare.io/micbot/internal/config"
	"medishare.io/micbot/internal/models"
	"medishare.io/micbot/internal/recorder"

	"github.com/denisbrodbeck/machineid"
	"github.com/nats-io/nats.go"
)

var (
	globalRecorder *recorder.Recorder
)

// Agent represents the stateful worker
type Agent struct {
	ID       string
	State    models.AgentState
	NatsConn *nats.Conn
	mu       sync.Mutex
	quit     chan struct{}
}

// 初始化录音器
func init_recorder() {
	config := recorder.RecorderConfig{
		OutputDir:   config.RecorderBasedir,
		FileFormat:  config.FileFormat,
		AudioDevice: config.AudioDevice, // Linux下通常为default或hw:0,0
		SampleRate:  config.SampleRate,
		Channels:    config.Channels,
		BitRate:     config.BitRate,
	}

	var err error
	globalRecorder, err = recorder.NewRecorder(config)
	if err != nil {
		log.Fatal(err)
	}
}

func NewAgent() *Agent {
	init_recorder()
	// id := uuid.New().String()
	id, err := machineid.ID()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(id)
	log.Printf("Starting Agent with Session ID: %s", id)
	return &Agent{
		ID:    id,
		State: models.StateIdle,
		quit:  make(chan struct{}),
	}
}

func (a *Agent) setState(newState models.AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	log.Printf("State transition: %s -> %s", a.State, newState)
	a.State = newState
}

// statusReporter 定期上报状态
func (a *Agent) statusReporter() {
	ticker := time.NewTicker(config.StatusInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.quit:
			return
		case <-ticker.C:
			a.reportStatus()
		}
	}
}

// reportStatus 实际执行状态上报
func (a *Agent) reportStatus() {
	a.mu.Lock()
	recorderState := globalRecorder.GetState()

	// Convert recorder state to AgentState
	var st models.AgentState
	switch recorderState {
	case "idle", "stopped":
		st = models.StateIdle
	case "recording", "starting", "active":
		st = models.StateRecording
	case "error":
		st = "error" // You might want to define this as a constant in models.go
	default:
		st = models.StateIdle // Default to idle
	}

	statusMsg := models.AgentStatus{
		SessionID:  a.ID,
		Status:     st,
		LastUpdate: time.Now(),
	}
	a.mu.Unlock()

	data, _ := json.Marshal(statusMsg)
	if err := a.NatsConn.Publish(config.StatusReport, data); err != nil {
		log.Printf("Error publishing status: %v", err)
	} else {
		log.Printf("Status reported: %s", statusMsg.Status)
	}
}

// --- NATS Command Handlers ---

func (a *Agent) handleStartRecord(m *nats.Msg) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.State == models.StateRecording {
		log.Println("Already recording.")
		return
	}

	// Mocking: Start recording operation
	log.Println("MOCK: Started recording...")
	outputFile, err := globalRecorder.Start()
	if err != nil {
		log.Fatalf("开始录音失败: %v", err)
	}
	fmt.Printf("开始录音，文件将保存到: %s\n", outputFile)
	a.State = models.StateRecording

	// Respond to the NATS request (if it was a request)
	if m.Reply != "" {
		a.NatsConn.Publish(m.Reply, []byte("Started recording successfully: "+outputFile))
	}
}

func (a *Agent) handleStopRecord(m *nats.Msg) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.State == models.StateIdle {
		log.Println("Not currently recording.")
		return
	}

	// Mocking: Stop recording operation
	log.Println("MOCK: Stopped recording.")
	// 停止录音
	if err := globalRecorder.Stop(); err != nil {
		log.Fatalf("停止录音失败: %v", err)
	}
	fmt.Println("录音已停止")
	a.State = models.StateIdle

	var mockFileName string
	files, err := globalRecorder.ListRecordings(1)
	if err != nil {
		log.Printf("获取录音文件列表失败: %v", err)
	} else {
		fmt.Println("最新录音文件:")
		for i, file := range files {
			mockFileName = fmt.Sprintf("%s", file)
			fmt.Printf("%d. %s\n", i+1, mockFileName)
		}
	}

	// Mocking: Assume a file was created upon stop
	// mockFileName := fmt.Sprintf("recording_%s_%d.wav", a.ID[:4], time.Now().Unix())

	// Immediately queue an upload command
	_bs, err := os.ReadFile(mockFileName)
	if err != nil {
		log.Fatalf("抓取录音[%s]失败: %v", mockFileName, err)
	}
	uploadCmd := models.CommandMessage{Payload: mockFileName, Body: _bs}
	data, _ := json.Marshal(uploadCmd)
	a.NatsConn.Publish(config.CmdUploadRecord, data)
	log.Printf("MOCK: Publishing upload command for file: %s", mockFileName)

	// Respond to the NATS request
	if m.Reply != "" {
		a.NatsConn.Publish(m.Reply, []byte("Stopped recording and queued upload: "+mockFileName))
	}
}

func (a *Agent) handleUploadRecord(m *nats.Msg) {
	var cmd models.CommandMessage
	if err := json.Unmarshal(m.Data, &cmd); err != nil {
		log.Printf("Failed to unmarshal upload command: %v", err)
		return
	}

	fileName := cmd.Payload
	if fileName == "" {
		log.Println("Upload command missing filename.")
		return
	}

	body := cmd.Body
	if len(body) == 0 {
		log.Println("Upload command missing body.")
		return
	}

	// Mocking upload process
	log.Printf("MOCK: Uploading record file: %s ==> size: %d", fileName, len(body))
	// time.Sleep(1 * time.Second) // Simulate network/storage delay

	// Mocking success and logging to DB (Agent does not write to DB, Server does this typically,
	// but based on the prompt "upload_record" processing success is returned.
	// In a real system, the agent finishes upload and *reports* success back to the Server,
	// which then logs it to the DB. For simplicity, we just echo success here.)
	log.Printf("MOCK: Successfully processed record: %s ==> size: %d", fileName, len(body))

	// If the server needs the final metadata, it would use a reply subject here.
}

func main() {
	agent := NewAgent()

	// 1. Connect to NATS
	nc, err := nats.Connect(config.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	agent.NatsConn = nc

	log.Println("Agent connected to NATS.")

	// 2. Subscribe to commands
	nc.Subscribe(config.CmdStartRecord, agent.handleStartRecord)
	nc.Subscribe(config.CmdStopRecord, agent.handleStopRecord)
	nc.Subscribe(config.CmdUploadRecord, agent.handleUploadRecord) // Agent handles its own uploads

	// 3. Start status reporting loop
	go agent.statusReporter()

	defer globalRecorder.Cleanup()

	// 4. Block forever
	log.Println("Agent running, waiting for commands...")

	select {}

}
