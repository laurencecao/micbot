package baichuan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"medishare.io/micbot/internal/config"
)

// GenerateSOAPRequest Baichuan API请求结构
type GenerateSOAPRequest struct {
	Dialogue string `json:"dialogue"`
	History  string `json:"history,omitempty"`
}

// GenerateSOAPResponse Baichuan API响应结构
type GenerateSOAPResponse struct {
	Status string `json:"status"`
	Data   string `json:"data"`
}

// GenerateMedicalRecord 调用Baichuan API生成病历记录
func GenerateMedicalRecord(dialogue string, history string) (string, error) {
	// 准备请求数据
	requestData := GenerateSOAPRequest{
		Dialogue: dialogue,
		History:  history,
	}

	// 编码JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("编码请求数据失败: %v", err)
	}

	req, err := http.NewRequest("POST", config.StructApiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("服务返回错误状态: %s", resp.Status)
	}

	// 读取响应内容
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	var baichuanResp GenerateSOAPResponse
	err = json.Unmarshal(respBody, &baichuanResp)
	if err != nil {
		return "", fmt.Errorf("解析Baichuan响应失败: %v, 响应内容: %s", err, string(respBody))
	}

	// 检查响应状态
	if baichuanResp.Status != "success" {
		return "", fmt.Errorf("Baichuan服务返回失败状态: %s", baichuanResp.Status)
	}

	return baichuanResp.Data, nil
}
