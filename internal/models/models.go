package models

import "time"

// AgentState 用于 Agent 的状态机
type AgentState string

const (
	StateIdle      AgentState = "idle"
	StateRecording AgentState = "recording"
)

// AgentStatus 存储在 SQLite 的 agents 表中
type AgentStatus struct {
	SessionID  string     `json:"session_id"` // Agent 唯一标识 (e.g., UUID)
	Status     AgentState `json:"status"`     // 当前状态
	LastUpdate time.Time  `json:"last_update"`
}

// Recording 存储在 SQLite 的 recordings 表中
type Recording struct {
	ID             int       `json:"id"`
	FileName       string    `json:"file_name"`
	UploadTime     time.Time `json:"upload_time"`
	SizeKB         int       `json:"size_kb"`
	Transcript     string    `json:"transcript"`
	RelatedCommand string    `json:"related_command"`
}

// CommandMessage 用于 NATS 发送的指令
type CommandMessage struct {
	AgentID string `json:"agent_id,omitempty"` // 用于定向指令或状态报告
	Payload string `json:"payload,omitempty"`  // 额外参数 (如文件名)
	Body    []byte `json:"body,omitempty"`
}
