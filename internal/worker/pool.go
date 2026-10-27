// Package worker 后台工作池：从任务队列取任务→调MiniMax API→存结果。
// 模拟模式下不调API不烧额度，用短暂延时模拟生成过程。
package worker

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Hunte123/dramastudio/internal/minimax"
	"github.com/Hunte123/dramastudio/internal/task"
)

type Pool struct {
	Store  *task.Store
	Client *minimax.Client
	Log    *slog.Logger
	DataDir string
	Ctx    context.Context
}

// Start 启动n个后台协程，循环处理队列。
func (p *Pool) Start(n int) {
	for i := 0; i < n; i++ {
		go p.worker(i)
	}
	p.Log.Info("worker pool started", "workers", n)
}

func (p *Pool) worker(id int) {
	for {
		select {
		case <-p.Ctx.Done():
			return
		default:
		}
		t, err := p.Store.NextPending()
		if err != nil {
			time.Sleep(3 * time.Second) // 队列空，等一下
			continue
		}
		p.process(t)
	}
}

func (p *Pool) process(t *task.Task) {
	p.Log.Info("processing", "task_id", t.ID, "dry_run", p.Client.DryRun)

	// 读取首帧图
	var imgB64 string
	if t.ImagePath != "" {
		data, err := os.ReadFile(t.ImagePath)
		if err != nil {
			p.Store.UpdateStatus(t.ID, task.StatusFailed, "读图失败: "+err.Error(), "", "")
			return
		}
		imgB64 = base64.StdEncoding.EncodeToString(data)
	}

	// 提交生成任务
	remoteID, err := p.Client.Generate(t.Prompt, imgB64)
	if err != nil {
		p.Store.UpdateStatus(t.ID, task.StatusFailed, err.Error(), "", "")
		return
	}
	p.Store.UpdateStatus(t.ID, task.StatusRunning, "", "", remoteID)

	if p.Client.DryRun {
		// 模拟模式：等2秒假装在生成，然后标记完成
		time.Sleep(2 * time.Second)
		videoPath := filepath.Join(p.DataDir, "videos", fmt.Sprintf("task_%d_mock.mp4", t.ID))
		os.MkdirAll(filepath.Dir(videoPath), 0o755)
		os.WriteFile(videoPath, []byte("mock"), 0o644)
		p.Store.UpdateStatus(t.ID, task.StatusDone, "", videoPath, remoteID)
		p.Log.Info("dry-run done", "task_id", t.ID)
		return
	}

	// 轮询等待完成（真实模式）
	for i := 0; i < 80; i++ { // 最多20分钟
		select {
		case <-p.Ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
		url, done, err := p.Client.Poll(remoteID)
		if err != nil {
			p.Store.UpdateStatus(t.ID, task.StatusFailed, err.Error(), "", "")
			return
		}
		if done {
			videoPath := filepath.Join(p.DataDir, "videos", fmt.Sprintf("task_%d.mp4", t.ID))
			if err := p.Client.DownloadFile(url, videoPath); err != nil {
				p.Store.UpdateStatus(t.ID, task.StatusFailed, "下载失败: "+err.Error(), "", "")
				return
			}
			p.Store.UpdateStatus(t.ID, task.StatusDone, "", videoPath, remoteID)
			p.Log.Info("done", "task_id", t.ID)
			return
		}
	}
	p.Store.UpdateStatus(t.ID, task.StatusFailed, "超时", "", "")
}
