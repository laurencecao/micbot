package main

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"medishare.io/micbot/internal/recorder"
)

var (
	// 创建全局录音器实例
	globalRecorder *recorder.Recorder
)

// 初始化录音器
func init() {
	config := recorder.RecorderConfig{
		OutputDir:  "/root/mp3",
		FileFormat: "mp3",
	}

	var err error
	globalRecorder, err = recorder.NewRecorder(config)
	if err != nil {
		log.Fatal(err)
	}
}

func Example() {
	// 创建录音管理器配置
	config := recorder.RecorderConfig{
		OutputDir:   "/root/mp3", // 可以改为 /root/mp3
		AudioDevice: "hw:3,0",    // Linux下通常为default或hw:0,0
		FileFormat:  "mp3",
		SampleRate:  16000,
		Channels:    1,
		BitRate:     "128k",
	}

	// 创建录音器
	recorder, err := recorder.NewRecorder(config)
	if err != nil {
		log.Fatalf("创建录音器失败: %v", err)
	}
	defer recorder.Cleanup()

	// 检查当前状态
	fmt.Printf("当前状态: %s\n", recorder.GetState())

	// 开始录音
	outputFile, err := recorder.Start()
	if err != nil {
		log.Fatalf("开始录音失败: %v", err)
	}
	fmt.Printf("开始录音，文件将保存到: %s\n", outputFile)

	// 录音过程中检查状态
	fmt.Printf("录音中... 状态: %s\n", recorder.GetState())

	// 模拟录音一段时间
	time.Sleep(92 * time.Second)

	// 停止录音
	if err := recorder.Stop(); err != nil {
		log.Fatalf("停止录音失败: %v", err)
	}
	fmt.Println("录音已停止")

	// 获取最新的5个录音文件
	files, err := recorder.ListRecordings(5)
	if err != nil {
		log.Printf("获取录音文件列表失败: %v", err)
	} else {
		fmt.Println("最新录音文件:")
		for i, file := range files {
			fmt.Printf("%d. %s\n", i+1, filepath.Base(file))
		}
	}

	// 清理超过7天的旧录音
	if err := recorder.CleanupOldRecordings(7); err != nil {
		log.Printf("清理旧文件失败: %v", err)
	}
}

// 高级用法示例：集成到Web服务中
func ExampleWebIntegration() {

	// HTTP处理器示例
	/*
		func startRecordingHandler(w http.ResponseWriter, r *http.Request) {
			if globalRecorder.GetState() == recorder.StateRecording {
				http.Error(w, "已经在录音中", http.StatusConflict)
				return
			}

			filePath, err := globalRecorder.Start()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			response := map[string]string{
				"status":   "recording",
				"filePath": filePath,
			}
			json.NewEncoder(w).Encode(response)
		}

		func stopRecordingHandler(w http.ResponseWriter, r *http.Request) {
			if err := globalRecorder.Stop(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			response := map[string]string{"status": "stopped"}
			json.NewEncoder(w).Encode(response)
		}

		func listRecordingsHandler(w http.ResponseWriter, r *http.Request) {
			n := 10 // 默认返回10个
			if nStr := r.URL.Query().Get("limit"); nStr != "" {
				if nInt, err := strconv.Atoi(nStr); err == nil && nInt > 0 {
					n = nInt
				}
			}

			files, err := globalRecorder.ListRecordings(n)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			type recordingInfo struct {
				FileName string    `json:"filename"`
				FullPath string    `json:"full_path"`
				Time     time.Time `json:"time"`
			}

			var recordings []recordingInfo
			for _, file := range files {
				info, err := os.Stat(file)
				if err != nil {
					continue
				}

				recordings = append(recordings, recordingInfo{
					FileName: filepath.Base(file),
					FullPath: file,
					Time:     info.ModTime(),
				})
			}

			json.NewEncoder(w).Encode(recordings)
		}
	*/
}

func main() {
	Example()
}
