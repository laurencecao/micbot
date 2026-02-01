package main

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"medishare.io/micbot/internal/asr"
	"medishare.io/micbot/internal/baichuan"
	"medishare.io/micbot/internal/config"
	"medishare.io/micbot/internal/database"
	"medishare.io/micbot/internal/models"

	"github.com/nats-io/nats.go"
)

var (
	reBold1 = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reBold2 = regexp.MustCompile(`__(.+?)__`)
	// Use \S to ensure the first character after * is not a space
	reItalic1 = regexp.MustCompile(`\*(\S.+?)\*`)
	reItalic2 = regexp.MustCompile(`_(\S.+?)_`)
	reCode    = regexp.MustCompile("`([^`]+?)`")
	reLink    = regexp.MustCompile(`\[([^\]]+?)\]\(([^)]+?)\)`)
)

// markdownToHTML将markdown格式的文本转换为HTML
func markdownToHTML(markdown string) string {
	if markdown == "" {
		return ""
	}

	var result strings.Builder
	lines := strings.Split(markdown, "\n")
	inParagraph := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "" {
			if inParagraph {
				result.WriteString("</p>")
				inParagraph = false
			}
			result.WriteString("<br>")
			continue
		}

		if strings.HasPrefix(trimmedLine, "#") {
			if inParagraph {
				result.WriteString("</p>")
				inParagraph = false
			}

			level := 0
			for _, ch := range trimmedLine {
				if ch == '#' {
					level++
				} else {
					break
				}
			}
			if level > 6 {
				level = 6
			}
			content := strings.TrimSpace(trimmedLine[level:])
			result.WriteString(fmt.Sprintf("<h%d>%s</h%d>", level, html.EscapeString(content), level))
			continue
		}

		if strings.HasPrefix(trimmedLine, "- ") || strings.HasPrefix(trimmedLine, "* ") {
			if inParagraph {
				result.WriteString("</p>")
				inParagraph = false
			}

			if i == 0 || !strings.HasPrefix(strings.TrimSpace(lines[i-1]), "- ") && !strings.HasPrefix(strings.TrimSpace(lines[i-1]), "* ") {
				result.WriteString("<ul>")
			}

			content := strings.TrimSpace(trimmedLine[2:])
			result.WriteString(fmt.Sprintf("<li>%s</li>", processInlineMarkdown(content)))

			if i+1 >= len(lines) || (!strings.HasPrefix(strings.TrimSpace(lines[i+1]), "- ") &&
				!strings.HasPrefix(strings.TrimSpace(lines[i+1]), "* ")) {
				result.WriteString("</ul>")
			}
			continue
		}

		if match := regexp.MustCompile(`^(\d+)\.\s+`).FindStringSubmatch(trimmedLine); match != nil {
			if inParagraph {
				result.WriteString("</p>")
				inParagraph = false
			}

			if i == 0 || !regexp.MustCompile(`^\d+\.\s+`).MatchString(strings.TrimSpace(lines[i-1])) {
				result.WriteString("<ol>")
			}

			content := regexp.MustCompile(`^\d+\.\s+`).ReplaceAllString(trimmedLine, "")
			result.WriteString(fmt.Sprintf("<li>%s</li>", processInlineMarkdown(content)))

			if i+1 >= len(lines) || !regexp.MustCompile(`^\d+\.\s+`).MatchString(strings.TrimSpace(lines[i+1])) {
				result.WriteString("</ol>")
			}
			continue
		}

		if strings.HasPrefix(trimmedLine, "> ") {
			if inParagraph {
				result.WriteString("</p>")
				inParagraph = false
			}

			content := strings.TrimSpace(trimmedLine[2:])
			result.WriteString(fmt.Sprintf("<blockquote>%s</blockquote>", html.EscapeString(content)))
			continue
		}

		if strings.HasPrefix(trimmedLine, "```") {
			if inParagraph {
				result.WriteString("</p>")
				inParagraph = false
			}

			lang := ""
			if len(trimmedLine) > 3 {
				lang = strings.TrimSpace(trimmedLine[3:])
			}
			result.WriteString(fmt.Sprintf("<pre%s><code>", formatCodeClass(lang)))

			for i++; i < len(lines); i++ {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					break
				}
				result.WriteString(html.EscapeString(lines[i]))
				result.WriteString("\n")
			}

			result.WriteString("</code></pre>")
			continue
		}

		if !inParagraph {
			result.WriteString("<p>")
			inParagraph = true
		} else {
			result.WriteString(" ")
		}

		result.WriteString(processInlineMarkdown(line))
	}

	if inParagraph {
		result.WriteString("</p>")
	}

	return result.String()
}

// processInlineMarkdown处理行内markdown格式
func processInlineMarkdown(text string) string {
	result := html.EscapeString(text)
	result = reBold1.ReplaceAllString(result, "<strong>$1</strong>")
	result = reBold2.ReplaceAllString(result, "<strong>$1</strong>")
	result = reItalic1.ReplaceAllString(result, "<em>$1</em>")
	result = reItalic2.ReplaceAllString(result, "<em>$1</em>")
	result = reCode.ReplaceAllString(result, "<code>$1</code>")
	result = reLink.ReplaceAllString(result, `<a href="$2" target="_blank">$1</a>`)
	return result
}

// formatCodeClass格式化代码块的语言类
func formatCodeClass(lang string) string {
	if lang == "" {
		return ""
	}
	return fmt.Sprintf(" class=\"language-%s\"", html.EscapeString(lang))
}

// extractTextFromTranscript 从Transcript JSON字符串中提取并合并所有text字段
func extractTextFromTranscript(transcript string) string {
	if transcript == "" {
		return ""
	}

	// Transcript字段存储的是raw_segments的JSON数组
	var rawSegments []interface{}
	if err := json.Unmarshal([]byte(transcript), &rawSegments); err != nil {
		log.Printf("Failed to parse transcript JSON: %v", err)
		return transcript
	}

	// 合并所有text字段
	var result strings.Builder
	for _, segment := range rawSegments {
		// 尝试将segment转换为map[string]interface{}
		if segmentMap, ok := segment.(map[string]interface{}); ok {
			if textValue, exists := segmentMap["text"]; exists {
				if text, ok := textValue.(string); ok && text != "" {
					if result.Len() > 0 {
						result.WriteString(" ")
					}
					result.WriteString(text)
				}
			}
		}
	}

	return result.String()
}

// filterThinkFromMedicalRecord 过滤掉MedicalRecord中think部分的内容
func filterThinkFromMedicalRecord(medicalRecord string) string {
	if medicalRecord == "" {
		return ""
	}

	// 首先过滤 <think>...</think> 标签及其内容
	// 使用正则表达式匹配 所有 <think> 和 </think> 之间的内容（包括标签）
	// 注意：需要处理多行匹配和嵌套标签的情况
	filteredText := ""

	insideThink := false
	var result strings.Builder

	// 按行处理，但需要跨行处理think标签
	lines := strings.Split(medicalRecord, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if !insideThink {
			// 检查是否开始think标签
			if strings.Contains(line, "<think>") {
				// 找到 <think> 标签开始位置
				startPos := strings.Index(line, "<think>")

				if startPos >= 0 {
					// 将think标签之前的内容写入结果
					result.WriteString(line[:startPos])

					// 检查think标签内是否有闭合标签
					remainingLine := line[startPos+len("<think>"):]

					// 检查是否有闭合标签在同一行
					endPos := strings.Index(remainingLine, "</think>")
					if endPos >= 0 {
						// 同一行中闭合了think标签
						// 跳过think标签内的内容，继续写入think标签之后的内容
						result.WriteString(remainingLine[endPos+len("</think>"):])
						if i < len(lines)-1 {
							result.WriteString("\n")
						}
						// 继续处理下一行
						continue
					} else {
						// 没有在同一行找到闭合标签，进入think模式
						insideThink = true
						// 继续处理下一行
						continue
					}
				}
			} else {
				// 没有think标签，直接写入整行
				result.WriteString(line)
				if i < len(lines)-1 {
					result.WriteString("\n")
				}
			}
		} else {
			// 在think标签内，检查是否结束
			endPos := strings.Index(line, "</think>")
			if endPos >= 0 {
				// 找到闭合标签
				// 跳过think标签内的内容和关闭标签
				remainingLine := line[endPos+len("</think>"):]
				if remainingLine != "" {
					result.WriteString(remainingLine)
				}
				if i < len(lines)-1 {
					result.WriteString("\n")
				}
				insideThink = false
			} else {
				// 还没结束，继续在think内，跳过这一行
				continue
			}
		}
	}

	filteredText = result.String()

	// 过滤常见think模式：
	// 1. think: 开头的内容
	// 2. thought: 开头的内容
	// 3. {think: ...} JSON格式的内容

	lines = strings.Split(filteredText, "\n")
	var finalFilteredLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// 跳过think相关的行
		if strings.HasPrefix(trimmedLine, "think:") ||
			strings.HasPrefix(trimmedLine, "thought:") ||
			strings.HasPrefix(trimmedLine, "思考:") ||
			strings.HasPrefix(trimmedLine, "内部思考:") ||
			strings.HasPrefix(trimmedLine, "```thinking") ||
			strings.Contains(trimmedLine, "{think:") {
			continue
		}

		// 如果不是think相关的行，保留
		finalFilteredLines = append(finalFilteredLines, line)
	}

	return strings.Join(finalFilteredLines, "\n")
}

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

		// 在这里处理transcribe
		log.Println("假装在跟ASR模型交互，转写中......", len(cmd.Body))

		// 调用新的转录函数（包含说话者识别）
		speakerResult, err := asr.TranscribeWithSpeaker(cmd.Body)
		if err != nil {
			fmt.Printf("带说话者识别的转录失败: %v\n", err)
			// 如果新服务失败，尝试使用旧服务作为备选
			oldResult, oldErr := asr.Transcribe(cmd.Body)
			if oldErr != nil {
				fmt.Printf("旧转录服务也失败: %v\n", oldErr)
				return
			}

			txt := "转写失败了！"
			if oldResult.Success {
				txt = oldResult.Text
			}

			// MOCK: Generate metadata for the DB
			newRecord := models.Recording{
				FileName:       cmd.Payload,
				UploadTime:     time.Now(),
				SizeKB:         len(cmd.Body) / 1024,
				Transcript:     txt,
				Dialogue:       "", // 初始化空字符串
				MedicalRecord:  "", // 初始化空字符串
				RelatedCommand: "(暂时假的，新ASR服务失败)",
			}

			// 写入数据库
			if err := database.InsertRecording(newRecord); err != nil {
				log.Printf("DB error inserting new recording: %v", err)
			}
			return
		}

		fmt.Printf("带说话者识别的转录结果: %v\n", speakerResult)

		// 将segments转换为JSON字符串存储
		rawSegmentsStr := ""
		if len(speakerResult.RawSegments) > 0 {
			segmentsBytes, err := json.Marshal(speakerResult.RawSegments)
			if err == nil {
				rawSegmentsStr = string(segmentsBytes)
			}
		}

		// 使用新的ASR结果填充数据库记录（先创建基础记录）
		newRecord := models.Recording{
			FileName:       cmd.Payload,
			UploadTime:     time.Now(),
			SizeKB:         len(cmd.Body) / 1024,
			Transcript:     rawSegmentsStr,             // segments JSON放在Transcript列
			Dialogue:       speakerResult.ToMarkdown(), // markdown格式文本放在Dialogue列
			MedicalRecord:  "",                         // 初始化空字符串，后面会填充
			RelatedCommand: "(新ASR服务完成，等待Baichuan处理)",
		}

		// 先插入基础记录到数据库
		if err := database.InsertRecording(newRecord); err != nil {
			log.Printf("DB error inserting new recording: %v", err)
		}

		// 不再自动调用Baichuan服务，等待用户上传医学检验结果
		// 将相关命令标记为等待医学检验结果
		err = database.UpdateRecordingMedicalRecord(cmd.Payload, "等待上传医学检验结果")
		if err != nil {
			log.Printf("更新相关命令字段失败: %v", err)
		} else {
			log.Printf("录音处理完成，等待医学检验结果上传，文件名: %s", cmd.Payload)
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

	// 处理后端数据处理：处理Transcript和MedicalRecord字段
	processedHistory := make([]models.Recording, len(history))
	for i, record := range history {
		processedRecord := record

		// 1. 处理Transcript字段：提取并合并raw_segments中的所有text字段
		processedRecord.Transcript = extractTextFromTranscript(record.Transcript)

		// 2. 过滤MedicalRecord中的think部分
		cleanedMedicalRecord := filterThinkFromMedicalRecord(record.MedicalRecord)

		// 3. 将markdown转换为HTML格式
		processedRecord.MedicalRecord = markdownToHTML(cleanedMedicalRecord)

		processedHistory[i] = processedRecord
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(processedHistory); err != nil {
		log.Printf("Error encoding history JSON: %v", err)
	}
}

func uploadMedicalChecksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	fileName := r.FormValue("file_name")
	if fileName == "" {
		http.Error(w, "File name is required", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("medical_checks_file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	fileContent := string(fileBytes)
	medicalChecksHTML := markdownToHTML(fileContent)

	err = database.UpdateRecordingMedicalChecks(fileName, medicalChecksHTML)
	if err != nil {
		log.Printf("Failed to update medical checks in database: %v", err)
		http.Error(w, "Failed to update medical checks", http.StatusInternalServerError)
		return
	}

	recordings, err := database.GetRecentRecordings(100)
	if err != nil {
		log.Printf("Failed to get recordings: %v", err)
		http.Error(w, "Failed to get dialogue", http.StatusInternalServerError)
		return
	}

	var dialogue string
	for _, rec := range recordings {
		if rec.FileName == fileName {
			dialogue = rec.Dialogue
			break
		}
	}

	if dialogue == "" {
		log.Printf("No dialogue found for file: %s", fileName)
		http.Error(w, "No dialogue found for this file", http.StatusNotFound)
		return
	}

	go func(dialogueText string, medicalChecksText string, fileName string) {
		log.Println("开始调用Baichuan服务生成病历记录，包含医学检验结果...")

		medicalRecordText, err := baichuan.GenerateMedicalRecord(dialogueText, medicalChecksText)
		if err != nil {
			log.Printf("Baichuan服务调用失败: %v", err)
			medicalRecordText = fmt.Sprintf("Baichuan服务调用失败: %v\n\n医学检验结果:\n%s", err, medicalChecksText)
		} else {
			log.Println("Baichuan服务调用成功")
		}

		err = database.UpdateRecordingMedicalRecord(fileName, medicalRecordText)
		if err != nil {
			log.Printf("更新病历记录到数据库失败: %v", err)
		} else {
			log.Printf("成功更新病历记录到数据库，文件名: %s", fileName)
		}
	}(dialogue, fileContent, fileName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Medical checks uploaded successfully and sent to Baichuan for processing",
	})
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

var mobileTmpl *template.Template

func initMobile() {
	mobileTmpl = template.Must(template.ParseFiles("web/mobile.html"))
}

func mobileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := mobileTmpl.Execute(w, nil); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

func withMobileCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler(w, r)
	}
}

func mobileRecordsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	records, err := database.GetMobileRecords()
	if err != nil {
		log.Printf("Query error: %v\n", err)
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		return
	}

	// 将markdown转换为HTML
	for i := range records {
		if records[i].AudioText != "" {
			records[i].AudioText = markdownToHTML(records[i].AudioText)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(records); err != nil {
		log.Printf("JSON encode error: %v\n", err)
	}
}

func mobileUploadHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[MOBILE UPLOAD] Starting upload handler\n")

	if r.Method != http.MethodPost {
		log.Printf("[MOBILE UPLOAD] Method not allowed: %s\n", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		log.Printf("[MOBILE UPLOAD] Failed to parse form: %v\n", err)
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, handler, err := r.FormFile("audio")
	if err != nil {
		log.Printf("[MOBILE UPLOAD] Failed to get file: %v\n", err)
		http.Error(w, "Failed to get file: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[MOBILE UPLOAD] Received file: %s, size: %d bytes\n", handler.Filename, handler.Size)

	timestamp := time.Now().Format("20060102_150405")
	originalFilename := fmt.Sprintf("temp_%s_%s", timestamp, handler.Filename)
	originalPath := filepath.Join(config.RecorderBasedir, originalFilename)

	log.Printf("[MOBILE UPLOAD] Target directory: %s\n", config.RecorderBasedir)

	if err := os.MkdirAll(config.RecorderBasedir, 0755); err != nil {
		file.Close()
		log.Printf("[MOBILE UPLOAD] Failed to create directory: %v\n", err)
		http.Error(w, "Failed to create directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	dst, err := os.Create(originalPath)
	if err != nil {
		file.Close()
		log.Printf("[MOBILE UPLOAD] Failed to create file: %v\n", err)
		http.Error(w, "Failed to create file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	written, err := io.Copy(dst, file)
	file.Close()
	dst.Close()

	if err != nil {
		os.Remove(originalPath)
		log.Printf("[MOBILE UPLOAD] Failed to save file: %v\n", err)
		http.Error(w, "Failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[MOBILE UPLOAD] File saved successfully: %s (%d bytes)\n", originalPath, written)

	mp3Filename := fmt.Sprintf("mobile_recording_%s.mp3", timestamp)
	mp3Path := filepath.Join(config.RecorderBasedir, mp3Filename)

	if err := convertToMP3(originalPath, mp3Path); err != nil {
		log.Printf("[MOBILE UPLOAD] MP3 conversion failed: %v, keeping original format\n", err)
		mp3Filename = originalFilename
		mp3Path = originalPath
	} else {
		log.Printf("[MOBILE UPLOAD] MP3 conversion successful: %s\n", mp3Path)
		os.Remove(originalPath)
	}

	log.Printf("[MOBILE UPLOAD] Inserting into database: %s\n", mp3Filename)
	id, err := database.InsertMobileRecording(mp3Filename)
	if err != nil {
		os.Remove(mp3Path)
		log.Printf("[MOBILE UPLOAD] Failed to insert record: %v\n", err)
		http.Error(w, "Failed to insert record: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[MOBILE UPLOAD] Success: id=%d, file=%s\n", id, mp3Filename)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      id,
		"message": "Upload successful",
	})
}

func convertToMP3(inputPath, outputPath string) error {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	cmd := exec.Command("ffmpeg", "-i", inputPath, "-vn", "-ar", "44100", "-ac", "2", "-b:a", "192k", "-y", outputPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %v, output: %s", err, string(output))
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("output file not created")
	}

	return nil
}

func mobileUpdateDiagnosisHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID      int    `json:"id"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := database.UpdateMobileDiagnosis(req.ID, req.Content); err != nil {
		http.Error(w, "Failed to update: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Diagnosis record updated",
	})
}

func mobileAudioHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Path[len("/mobile/audio/"):]
	if filename == "" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	audioPath := filepath.Join(config.RecorderBasedir, filename)

	http.ServeFile(w, r, audioPath)
}

func mobileTranscribeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	record, err := database.GetMobileRecordByID(req.ID)
	if err != nil {
		log.Printf("Failed to get record: %v", err)
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	if record.AudioFile == "" {
		http.Error(w, "No audio file associated with this record", http.StatusBadRequest)
		return
	}

	audioPath := filepath.Join(config.RecorderBasedir, record.AudioFile)
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		log.Printf("Failed to read audio file: %v", err)
		http.Error(w, "Failed to read audio file", http.StatusInternalServerError)
		return
	}

	result, err := asr.TranscribeWithSpeaker(audioData)
	if err != nil {
		log.Printf("Transcription failed: %v", err)
		http.Error(w, "Transcription failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	markdownText := result.ToMarkdown()
	if err := database.UpdateMobileAudioText(req.ID, markdownText); err != nil {
		log.Printf("Failed to save transcript: %v", err)
		http.Error(w, "Failed to save transcript", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"transcript": markdownText,
		"message":    "Transcription completed successfully",
	})
}

func main() {
	config.LoadConfigForMe()

	if _, err := os.Stat("web/index.html"); os.IsNotExist(err) {
		log.Fatal("web/index.html not found. Please create the frontend template.")
	}

	initServer()
	initMobile()
	defer natsConn.Close()

	subscribeAgentStatus()
	subscribeUploadRecord()

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/api/command", commandHandler)
	http.HandleFunc("/api/status", apiStatusHandler)
	http.HandleFunc("/api/history", apiHistoryHandler)
	http.HandleFunc("/api/upload_medical_checks", uploadMedicalChecksHandler)

	http.HandleFunc("/mobile", mobileHandler)
	http.HandleFunc("/api/mobile/records", withMobileCORS(mobileRecordsHandler))
	http.HandleFunc("/api/mobile/records/upload", withMobileCORS(mobileUploadHandler))
	http.HandleFunc("/api/mobile/records/update-diagnosis", withMobileCORS(mobileUpdateDiagnosisHandler))
	http.HandleFunc("/api/mobile/records/transcribe", withMobileCORS(mobileTranscribeHandler))
	http.HandleFunc("/mobile/audio/", withMobileCORS(mobileAudioHandler))

	http.Handle("/static/", withLoggingStatic(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static")))))

	port := ":8080"
	certFile := config.CertFile
	keyFile := config.KeyFile

	if config.EnableSSL {
		if certFile != "" && keyFile != "" {
			if _, err := os.Stat(certFile); err == nil {
				if _, err := os.Stat(keyFile); err == nil {
					log.Printf("Web Server running on https://localhost%s", port)
					if err := http.ListenAndServeTLS(port, certFile, keyFile, nil); err != nil {
						log.Fatal(err)
					}
					return
				}
			}
			log.Printf("警告: enable_ssl=true 但证书文件不存在，回退到 HTTP")
		} else {
			log.Printf("警告: enable_ssl=true 但证书路径未配置，回退到 HTTP")
		}
	}

	log.Printf("Web Server running on http://localhost%s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func withLoggingStatic(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler.ServeHTTP(w, r)
		log.Printf("[STATIC] %s %s - %v\n", r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}
