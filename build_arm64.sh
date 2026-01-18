GOOS=linux GOARCH=arm64 go build -o bin/webserver_arm64 cmd/webserver/main.go
GOOS=linux GOARCH=arm64 go build -o bin/micbot_arm64 cmd/agent/main.go
GOOS=linux GOARCH=arm64 go build -o bin/recorder_arm64 cmd/recorder/main.go