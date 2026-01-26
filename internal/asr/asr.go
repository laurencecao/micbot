package asr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"

	"medishare.io/micbot/internal/config"
)

type Response struct {
	Filename     string `json:"filename,omitempty"`
	Success      bool   `json:"success,omitempty"`
	Text         string `json:"text,omitempty"`
	RawSegments  string `json:"raw_segments,omitempty"`
	Transcript   string `json:"transcript,omitempty"`
	DetectedLang string `json:"detected_language,omitempty"`
}

func parseResult(jsonStr string) (Response, error) {
	var resp Response
	err := json.Unmarshal([]byte(jsonStr), &resp)
	if err != nil {
		fmt.Printf("解析JSON失败: %v\n", err)
		return resp, err
	}
	resp.Success = true
	return resp, nil
}

// ensure_wave 检测音频格式并转换为 WAV 格式
func ensure_wave(audioData []byte) ([]byte, error) {
	// 首先尝试检测音频格式
	// 这里使用 file 命令检测文件类型，也可以使用专门的音频库如 go-ffmpeg 等
	// 为简化示例，这里使用简单的文件头检测和 FFmpeg 转换

	// 临时文件路径
	tempInput := "temp_input"
	tempOutput := "temp_output.wav"

	// 写入临时文件
	if err := os.WriteFile(tempInput, audioData, 0644); err != nil {
		return nil, fmt.Errorf("写入临时文件失败: %v", err)
	}
	defer os.Remove(tempInput)
	defer os.Remove(tempOutput)

	// 检查是否需要转换（简化的格式检测）
	needConversion := false

	// 检查常见的音频格式文件头
	if len(audioData) >= 4 {
		// MP3 格式（ID3 标签或 MPEG 帧）
		if bytes.HasPrefix(audioData, []byte{0x49, 0x44, 0x33}) || // ID3
			bytes.Equal(audioData[:3], []byte{0xFF, 0xFB, 0x90}) { // MPEG
			needConversion = true
		}
		// FLAC 格式
		if bytes.Equal(audioData[:4], []byte{'f', 'L', 'a', 'C'}) {
			needConversion = true
		}
		// OGG 格式
		if bytes.Equal(audioData[:4], []byte{'O', 'g', 'g', 'S'}) {
			needConversion = true
		}
		// WAV 格式检查
		if bytes.Equal(audioData[:4], []byte{'R', 'I', 'F', 'F'}) {
			// 已经是 WAV 格式，不需要转换
			return audioData, nil
		}
	}

	// 如果不是上述格式，尝试使用 FFmpeg 检测
	if !needConversion {
		// 这里可以添加更复杂的检测逻辑
		// 简化为：如果不是 WAV 格式，就尝试转换
		needConversion = true
	}

	if !needConversion {
		return audioData, nil
	}

	// 使用 FFmpeg 进行转换
	cmd := exec.Command("ffmpeg",
		"-i", tempInput, // 输入文件
		"-acodec", "pcm_s16le", // PCM 16-bit little-endian
		"-ar", "16000", // 采样率 16kHz（通常用于语音识别）
		"-ac", "1", // 单声道
		"-y", // 覆盖输出文件
		tempOutput)

	// 执行转换
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("音频转换失败: %v", err)
	}

	// 读取转换后的 WAV 文件
	wavData, err := os.ReadFile(tempOutput)
	if err != nil {
		return nil, fmt.Errorf("读取转换后的文件失败: %v", err)
	}

	return wavData, nil
}

// transcribe 提交音频数据到转录服务
func Transcribe(audioData []byte) (Response, error) {
	// 首先确保音频是 WAV 格式
	wavData, err := ensure_wave(audioData)
	if err != nil {
		return Response{}, fmt.Errorf("音频格式转换失败: %v", err)
	}

	// 创建 multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 创建表单文件部分
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return Response{}, fmt.Errorf("创建表单文件失败: %v", err)
	}

	// 写入音频数据
	if _, err := io.Copy(part, bytes.NewReader(wavData)); err != nil {
		return Response{}, fmt.Errorf("写入音频数据失败: %v", err)
	}

	// 关闭 writer 以写入结束边界
	if err := writer.Close(); err != nil {
		return Response{}, fmt.Errorf("关闭表单写入器失败: %v", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", config.ASRApiURL, body)
	if err != nil {
		return Response{}, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置 Content-Type 头部
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("服务返回错误状态: %s", resp.Status)
	}

	// 读取响应内容
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("读取响应失败: %v", err)
	}

	ret := string(respBody)
	return parseResult(ret)
}

// 示例使用函数
func exampleUsage() {
	// 读取音频文件
	audioData, err := os.ReadFile("audio.mp3")
	if err != nil {
		fmt.Printf("读取文件失败: %v\n", err)
		return
	}

	// 调用转录函数
	result, err := Transcribe(audioData)
	if err != nil {
		fmt.Printf("转录失败: %v\n", err)
		return
	}

	fmt.Printf("转录结果: %s\n", result)
}

// 纯 Go 实现的格式检测（无外部依赖）
func detectAudioFormat(data []byte) string {
	if len(data) < 4 {
		return "unknown"
	}

	// WAV: RIFF
	if bytes.Equal(data[:4], []byte{'R', 'I', 'F', 'F'}) {
		return "wav"
	}

	// MP3: ID3 tag
	if bytes.HasPrefix(data, []byte{0x49, 0x44, 0x33}) {
		return "mp3"
	}

	// FLAC
	if bytes.Equal(data[:4], []byte{'f', 'L', 'a', 'C'}) {
		return "flac"
	}

	// OGG
	if bytes.Equal(data[:4], []byte{'O', 'g', 'g', 'S'}) {
		return "ogg"
	}

	// AAC/MP4
	if bytes.HasPrefix(data, []byte{0x00, 0x00, 0x00}) && len(data) > 8 {
		if bytes.Equal(data[4:8], []byte{'f', 't', 'y', 'p'}) {
			return "aac/mp4"
		}
	}

	return "unknown"
}

// 纯 Go 实现的 ensure_wave 替代版本（需要安装 FFmpeg）
func ensure_wave_ffmpeg(audioData []byte) ([]byte, error) {
	// 格式检测
	format := detectAudioFormat(audioData)
	fmt.Printf("检测到音频格式: %s\n", format)

	// 如果是 WAV 格式，直接返回
	if format == "wav" {
		return audioData, nil
	}

	// 否则进行转换（需要 FFmpeg）
	return ensure_wave(audioData)
}

func main() {
	// 运行示例
	exampleUsage()
}
