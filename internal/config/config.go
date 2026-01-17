package config

import "time"

const (
	NatsURL = "nats://10.0.2.150:14222"
	DBPath  = "recorder.db"

	// NATS Subjects
	CmdStartRecord  = "command.record.start"
	CmdStopRecord   = "command.record.stop"
	CmdUploadRecord = "command.record.upload"
	StatusReport    = "status.agent.report" // Agent -> Server status updates

	// Agent Status Reporting Interval
	StatusInterval = 10 * time.Second
)
