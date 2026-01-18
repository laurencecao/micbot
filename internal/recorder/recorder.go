package recorder

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Recorder 录音管理器
type Recorder struct {
	mu          sync.RWMutex
	state       State
	outputDir   string
	currentCmd  *exec.Cmd
	ctx         context.Context
	cancel      context.CancelFunc
	audioDevice string
	fileFormat  string
	sampleRate  int
	channels    int
	bitRate     string
}

// State 录音状态
type State string

const (
	StateIdle      State = "idle"
	StateRecording State = "recording"
	StateError     State = "error"
)

// RecorderConfig 录音配置
type RecorderConfig struct {
	OutputDir   string // 输出目录，默认 /root/mp3
	AudioDevice string // 音频设备，默认 default
	FileFormat  string // 文件格式，默认 mp3
	SampleRate  int    // 采样率，默认 44100
	Channels    int    // 声道数，默认 2
	BitRate     string // 比特率，默认 128k
}

// NewRecorder 创建新的录音管理器
func NewRecorder(cfg RecorderConfig) (*Recorder, error) {
	// 设置默认值
	if cfg.OutputDir == "" {
		cfg.OutputDir = "/root/mp3"
	}
	if cfg.AudioDevice == "" {
		cfg.AudioDevice = "default"
	}
	if cfg.FileFormat == "" {
		cfg.FileFormat = "mp3"
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 44100
	}
	if cfg.Channels == 0 {
		cfg.Channels = 2
	}
	if cfg.BitRate == "" {
		cfg.BitRate = "128k"
	}

	// 确保输出目录存在
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 检查ffmpeg是否可用
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg未找到，请先安装: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Recorder{
		state:       StateIdle,
		outputDir:   cfg.OutputDir,
		ctx:         ctx,
		cancel:      cancel,
		audioDevice: cfg.AudioDevice,
		fileFormat:  strings.ToLower(cfg.FileFormat),
		sampleRate:  cfg.SampleRate,
		channels:    cfg.Channels,
		bitRate:     cfg.BitRate,
	}, nil
}

// Start 开始录音
func (r *Recorder) Start() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == StateRecording {
		return "", fmt.Errorf("已经在录音中")
	}

	// 生成文件名（时间戳）
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("recording_%s.%s", timestamp, r.fileFormat)
	outputFile := filepath.Join(r.outputDir, filename)

	// 构建ffmpeg命令
	args := []string{
		"-f", "alsa", // ALSA音频输入
		"-i", r.audioDevice, // 音频设备
		"-ar", fmt.Sprintf("%d", r.sampleRate), // 采样率
		"-ac", fmt.Sprintf("%d", r.channels), // 声道数
		"-b:a", r.bitRate, // 音频比特率
		"-y", // 覆盖输出文件
	}

	// 根据格式添加额外参数
	switch r.fileFormat {
	case "mp3":
		args = append(args, "-codec:a", "libmp3lame")
	case "wav":
		args = append(args, "-codec:a", "pcm_s16le")
	case "aac":
		args = append(args, "-codec:a", "aac")
	case "flac":
		args = append(args, "-codec:a", "flac")
	}

	args = append(args, outputFile)

	// 创建命令
	cmd := exec.CommandContext(r.ctx, "ffmpeg", args...)
	cmd.Stderr = os.Stderr // 输出错误信息到标准错误

	// 启动录音进程
	if err := cmd.Start(); err != nil {
		r.state = StateError
		return "", fmt.Errorf("启动录音失败: %w", err)
	}

	r.currentCmd = cmd
	r.state = StateRecording

	// 启动goroutine监控进程状态
	go r.monitorProcess(cmd, outputFile)

	return outputFile, nil
}

// Stop 停止录音
func (r *Recorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != StateRecording {
		return fmt.Errorf("没有在录音")
	}

	// 取消上下文，终止进程
	if r.cancel != nil {
		r.cancel()
	}

	// 等待进程结束
	if r.currentCmd != nil && r.currentCmd.Process != nil {
		r.currentCmd.Process.Signal(os.Interrupt)
		time.Sleep(100 * time.Millisecond) // 短暂等待确保文件写入完成
	}

	// 重置状态
	r.reset()

	return nil
}

// GetState 获取当前状态
func (r *Recorder) GetState() State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// ListRecordings 获取最新的N个录音文件（按创建时间排序）
func (r *Recorder) ListRecordings(n int) ([]string, error) {
	var files []fileInfo

	err := filepath.WalkDir(r.outputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// 检查文件扩展名
		ext := strings.ToLower(filepath.Ext(path))
		validExts := map[string]bool{
			".mp3": true, ".wav": true, ".aac": true, ".flac": true, ".m4a": true,
		}

		if validExts[ext] {
			info, err := d.Info()
			if err != nil {
				return err
			}

			files = append(files, fileInfo{
				Path:    path,
				ModTime: info.ModTime(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("遍历目录失败: %w", err)
	}

	// 按修改时间倒序排序（最新的在前面）
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	// 获取前N个文件
	var result []string
	for i := 0; i < len(files) && i < n; i++ {
		result = append(result, files[i].Path)
	}

	return result, nil
}

// Cleanup 清理资源
func (r *Recorder) Cleanup() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 如果正在录音，先停止
	if r.state == StateRecording && r.cancel != nil {
		r.cancel()
	}

	return r.cleanupRecordings(0) // 0表示不清理
}

// CleanupOldRecordings 清理超过指定天数的录音文件
func (r *Recorder) CleanupOldRecordings(days int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cleanupRecordings(days)
}

// 内部方法：清理录音文件
func (r *Recorder) cleanupRecordings(days int) error {
	if days <= 0 {
		return nil // 不清理
	}

	cutoffTime := time.Now().AddDate(0, 0, -days)

	err := filepath.WalkDir(r.outputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if info.ModTime().Before(cutoffTime) {
			// 删除旧文件
			os.Remove(path)
		}

		return nil
	})

	return err
}

// 内部方法：重置状态
func (r *Recorder) reset() {
	// 创建新的上下文
	ctx, cancel := context.WithCancel(context.Background())
	r.ctx = ctx
	r.cancel = cancel
	r.currentCmd = nil
	r.state = StateIdle
}

// 内部方法：监控进程状态
func (r *Recorder) monitorProcess(cmd *exec.Cmd, outputFile string) {
	err := cmd.Wait()

	r.mu.Lock()
	defer r.mu.Unlock()

	// 只有当当前命令仍然是这个cmd时才更新状态
	if r.currentCmd == cmd {
		if err != nil {
			// 检查文件是否创建成功
			if _, statErr := os.Stat(outputFile); statErr == nil {
				// 文件已创建，可能是正常退出
				r.reset()
			} else {
				// 文件未创建，录音失败
				r.state = StateError
			}
		} else {
			// 正常结束
			r.reset()
		}
	}
}

// fileInfo 用于文件排序的结构体
type fileInfo struct {
	Path    string
	ModTime time.Time
}
