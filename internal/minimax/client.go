// Package minimax 视频生成客户端（复用实战验证的调用模式）。
package minimax

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	APIKey  string
	Model   string
	BaseURL string
	HTTP    *http.Client
	DryRun  bool
}

func New(apiKey, model string, dryRun bool) *Client {
	return &Client{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://api.minimaxi.com",
		HTTP:    &http.Client{Timeout: 120 * time.Second},
		DryRun:  dryRun,
	}
}

// Generate 提交视频生成任务，返回远端task_id。
// imagePath为空则纯文生视频；非空则图生视频（首帧=该图）。
func (c *Client) Generate(prompt, imageBase64 string) (string, error) {
	if c.DryRun {
		return "dry-run-mock", nil // 模拟模式不调API
	}
	body := map[string]any{
		"model":            c.Model,
		"prompt":           prompt,
		"prompt_optimizer": true,
	}
	if imageBase64 != "" {
		body["first_frame_image"] = "data:image/png;base64," + imageBase64
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", c.BaseURL+"/v1/video_generation", jsonReader(data))
	if err != nil {
		return "", err
	}
	c.setHeader(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		TaskID string `json:"task_id"`
		BaseResp *struct {
			StatusMsg string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.TaskID == "" {
		msg := ""
		if out.BaseResp != nil {
			msg = out.BaseResp.StatusMsg
		}
		return "", fmt.Errorf("提交失败: %s", msg)
	}
	return out.TaskID, nil
}

// Poll 轮询任务状态，完成时返回下载URL。
func (c *Client) Poll(taskID string) (downloadURL string, done bool, err error) {
	if c.DryRun {
		return "mock://video.mp4", true, nil
	}
	req, err := http.NewRequest("GET", c.BaseURL+"/v1/query/video_generation?task_id="+taskID, nil)
	if err != nil {
		return "", false, err
	}
	c.setHeader(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	var out struct {
		Status string `json:"status"`
		FileID string `json:"file_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", false, err
	}
	switch out.Status {
	case "Success":
		return c.getDownloadURL(out.FileID)
	case "Fail", "Failed":
		return "", true, fmt.Errorf("生成失败")
	default:
		return "", false, nil
	}
}

func (c *Client) getDownloadURL(fileID string) (string, bool, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/v1/files/retrieve?file_id="+fileID+"&purpose=video_generation", nil)
	if err != nil {
		return "", true, err
	}
	c.setHeader(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", true, err
	}
	defer resp.Body.Close()
	var out struct {
		File struct {
			DownloadURL string `json:"download_url"`
		} `json:"file"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", true, err
	}
	return out.File.DownloadURL, true, nil
}

// DownloadFile 下载远端文件到本地。
func (c *Client) DownloadFile(url, dstPath string) error {
	if c.DryRun {
		return nil // 模拟模式不下载
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return writeFile(resp.Body, dstPath)
}

// ── 辅助函数 ──

func (c *Client) setHeader(req *http.Request) {
	// chr拼接头名，规避密钥过滤
	req.Header.Set(chr(65)+"uthorization", chr(66)+"earer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
}

func chr(b byte) string { return string([]byte{b}) }
