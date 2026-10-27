// Package server HTTP路由与API处理器。
package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Hunte123/dramastudio/internal/config"
	"github.com/Hunte123/dramastudio/internal/task"
)

//go:embed all:static
var staticFS embed.FS

type Server struct {
	Cfg  *config.Config
	Store *task.Store
	Log  *slog.Logger
	mux  *http.ServeMux
}

func New(cfg *config.Config, store *task.Store, log *slog.Logger) *Server {
	s := &Server{Cfg: cfg, Store: store, Log: log, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	// API
	s.mux.HandleFunc("GET /api/tasks", s.handleList)
	s.mux.HandleFunc("POST /api/tasks", s.handleCreate)
	s.mux.HandleFunc("POST /api/tasks/batch", s.handleBatchCreate)
	s.mux.HandleFunc("DELETE /api/tasks/{id}", s.handleDelete)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("POST /api/upload", s.handleUpload)
	// 静态文件（嵌入的Web UI）
	s.mux.Handle("/", http.FileServerFS(staticFS))
	// 上传的图片和生成的视频
	s.mux.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(s.Cfg.DataDir))))
}

func (s *Server) Handler() http.Handler { return s.mux }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// ── 任务列表 ──

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.Store.ListAll()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if tasks == nil {
		tasks = []task.Task{}
	}
	writeJSON(w, 200, map[string]any{"tasks": tasks})
}

// ── 创建单个任务 ──

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt    string `json:"prompt"`
		ImagePath string `json:"image_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "请求体解析失败")
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		writeErr(w, 400, "prompt不能为空")
		return
	}
	id, err := s.Store.Create(&task.Task{Prompt: req.Prompt, ImagePath: req.ImagePath})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}

// ── 批量创建（JSON数组） ──

func (s *Server) handleBatchCreate(w http.ResponseWriter, r *http.Request) {
	var reqs []struct {
		Prompt    string `json:"prompt"`
		ImagePath string `json:"image_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		writeErr(w, 400, "批量导入格式错误，需要JSON数组")
		return
	}
	if len(reqs) == 0 {
		writeErr(w, 400, "数组为空")
		return
	}
	if len(reqs) > 50 {
		writeErr(w, 400, "单次最多50条")
		return
	}
	var ids []int64
	for _, req := range reqs {
		if strings.TrimSpace(req.Prompt) == "" {
			continue
		}
		id, err := s.Store.Create(&task.Task{Prompt: req.Prompt, ImagePath: req.ImagePath})
		if err != nil {
			writeErr(w, 500, fmt.Sprintf("第%d条创建失败: %v", len(ids)+1, err))
			return
		}
		ids = append(ids, id)
	}
	writeJSON(w, 201, map[string]any{"created": len(ids), "ids": ids})
}

// ── 删除任务 ──

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, 400, "缺少id")
		return
	}
	if err := s.Store.Delete(parseID(id)); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "deleted"})
}

// ── 统计 ──

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	counts, err := s.Store.CountByStatus()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"pending":  counts[task.StatusPending],
		"running":  counts[task.StatusRunning],
		"done":     counts[task.StatusDone],
		"failed":   counts[task.StatusFailed],
		"dry_run":  s.Cfg.DryRun,
		"model":    s.Cfg.VideoModel,
	})
}

// ── 图片上传 ──

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB
		writeErr(w, 400, "上传失败: "+err.Error())
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeErr(w, 400, "缺少image字段")
		return
	}
	defer file.Close()
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".png"
	}
	dst := filepath.Join(s.Cfg.DataDir, "uploads", fmt.Sprintf("%d%s", time.Now().UnixMilli(), ext))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	out, err := os.Create(dst)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	// 返回相对路径供前端引用
	rel, _ := filepath.Rel(s.Cfg.DataDir, dst)
	writeJSON(w, 201, map[string]string{"path": rel})
}

func parseID(s string) int64 {
	var id int64
	fmt.Sscanf(s, "%d", &id)
	return id
}
