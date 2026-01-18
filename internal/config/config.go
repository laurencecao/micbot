package config

import "time"

var (
	NatsURL = "nats://10.0.2.150:14222"
	DBPath  = "recorder.db"

	// NATS Subjects
	CmdStartRecord  = "command.record.start"
	CmdStopRecord   = "command.record.stop"
	CmdUploadRecord = "command.record.upload"
	StatusReport    = "status.agent.report" // Agent -> Server status updates

	// Agent Status Reporting Interval
	StatusInterval = 5 * time.Second

	RecorderBasedir = "/root/mp3"
	FileFormat      = "mp3"
	AudioDevice     = "hw:3,0"
	SampleRate      = 16000
	Channels        = 1
	BitRate         = "192k"
)
