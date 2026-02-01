package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"gopkg.in/ini.v1"
)

var (
	NatsURL      = "nats://10.0.2.150:14222"
	ASRApiURL    = "http://localhost:5000/transcribe"
	StructApiURL = "http://localhost:8000/generate_soap"
	DBPath       = "recorder.db"

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

	CertFile  = "./certs/cert.pem"
	KeyFile   = "./certs/key.pem"
	EnableSSL = false

	configLoaded = false
)

func LoadConfigForMe() {
	// 方式1: 加载现有配置文件
	err := LoadFromINI("config.ini")
	if err == nil {
		return
	}
	fmt.Printf("Error loading config: %v\n", err)

	// 方式2: 加载或创建默认配置（推荐）
	err = LoadFromINIOrCreateDefault("config.ini")
	if err == nil {
		return
	}
	fmt.Printf("Error: %v\n", err)

	// 检查配置是否已加载
	if IsLoaded() {
		fmt.Println("Configuration loaded successfully")
	} else {
		log.Println("Using default configuration")
	}
}

// LoadFromINI 从指定的INI文件加载配置
func LoadFromINI(configPath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist: %s", configPath)
	}
	// 加载INI文件
	cfg, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %v", err)
	}
	// 读取nats配置
	if section, err := cfg.GetSection("nats"); err == nil {
		if key, err := section.GetKey("url"); err == nil {
			NatsURL = key.String()
		}
	}
	// 读取asr配置
	if section, err := cfg.GetSection("asr"); err == nil {
		if key, err := section.GetKey("api_url"); err == nil {
			ASRApiURL = key.String()
		}
	}
	// 读取struct配置
	if section, err := cfg.GetSection("struct"); err == nil {
		if key, err := section.GetKey("api_url"); err == nil {
			StructApiURL = key.String()
		}
	}
	// 读取database配置
	if section, err := cfg.GetSection("database"); err == nil {
		if key, err := section.GetKey("path"); err == nil {
			DBPath = key.String()
		}
	}
	// 读取agent配置
	if section, err := cfg.GetSection("agent"); err == nil {
		if key, err := section.GetKey("status_interval"); err == nil {
			if seconds, err := key.Int(); err == nil && seconds > 0 {
				StatusInterval = time.Duration(seconds) * time.Second
			}
		}
	}
	// 读取recorder配置
	if section, err := cfg.GetSection("recorder"); err == nil {
		// 基础目录
		if key, err := section.GetKey("basedir"); err == nil {
			RecorderBasedir = key.String()
		}
		// 文件格式
		if key, err := section.GetKey("file_format"); err == nil {
			FileFormat = key.String()
		}
		// 音频设备
		if key, err := section.GetKey("audio_device"); err == nil {
			AudioDevice = key.String()
		}
		// 采样率
		if key, err := section.GetKey("sample_rate"); err == nil {
			if rate, err := key.Int(); err == nil && rate > 0 {
				SampleRate = rate
			}
		}
		// 声道数
		if key, err := section.GetKey("channels"); err == nil {
			if channels, err := key.Int(); err == nil && channels > 0 {
				Channels = channels
			}
		}
		// 比特率
		if key, err := section.GetKey("bit_rate"); err == nil {
			BitRate = key.String()
		}
	}
	if section, err := cfg.GetSection("webserver"); err == nil {
		if key, err := section.GetKey("cert_file"); err == nil {
			CertFile = key.String()
		}
		if key, err := section.GetKey("key_file"); err == nil {
			KeyFile = key.String()
		}
		if key, err := section.GetKey("enable_ssl"); err == nil {
			EnableSSL = key.MustBool(false)
		}
	}
	configLoaded = true
	return nil
}

// LoadFromINIOrCreateDefault 尝试从INI文件加载配置，如果文件不存在则创建默认配置
func LoadFromINIOrCreateDefault(configPath string) error {
	// 尝试加载配置
	err := LoadFromINI(configPath)
	if err == nil {
		return nil
	}
	// 如果文件不存在，创建默认配置
	if os.IsNotExist(err) {
		return createDefaultConfig(configPath)
	}
	// 其他错误
	return err
}

// createDefaultConfig 创建默认配置的INI文件
func createDefaultConfig(configPath string) error {
	cfg := ini.Empty()
	// 创建nats节
	natsSection, _ := cfg.NewSection("nats")
	natsSection.NewKey("url", NatsURL)
	// 创建asr节
	asrSection, _ := cfg.NewSection("asr")
	asrSection.NewKey("api_url", ASRApiURL)
	// 创建database节
	databaseSection, _ := cfg.NewSection("database")
	databaseSection.NewKey("path", DBPath)
	// 创建agent节
	agentSection, _ := cfg.NewSection("agent")
	agentSection.NewKey("status_interval", strconv.Itoa(int(StatusInterval.Seconds())))
	// 创建recorder节
	recorderSection, _ := cfg.NewSection("recorder")
	recorderSection.NewKey("basedir", RecorderBasedir)
	recorderSection.NewKey("file_format", FileFormat)
	recorderSection.NewKey("audio_device", AudioDevice)
	recorderSection.NewKey("sample_rate", strconv.Itoa(SampleRate))
	recorderSection.NewKey("channels", strconv.Itoa(Channels))
	recorderSection.NewKey("bit_rate", BitRate)
	webserverSection, _ := cfg.NewSection("webserver")
	webserverSection.NewKey("cert_file", CertFile)
	webserverSection.NewKey("key_file", KeyFile)
	webserverSection.NewKey("enable_ssl", strconv.FormatBool(EnableSSL))
	if err := cfg.SaveTo(configPath); err != nil {
		return fmt.Errorf("failed to save default config: %v", err)
	}
	fmt.Printf("Created default config file: %s\n", configPath)
	configLoaded = true
	return nil
}

// IsLoaded 返回配置是否已从INI文件加载
func IsLoaded() bool {
	return configLoaded
}
