// Package task 任务模型与SQLite持久化。
package task

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
)

type Task struct {
	ID          int64     `json:"id"`
	Prompt      string    `json:"prompt"`
	ImagePath   string    `json:"image_path"`  // 上传的首帧图路径
	VideoPath   string    `json:"video_path"`  // 生成的视频路径
	Status      Status    `json:"status"`
	Error       string    `json:"error,omitempty"`
	TaskID      string    `json:"task_id,omitempty"` // MiniMax远端task_id
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		prompt TEXT NOT NULL,
		image_path TEXT NOT NULL DEFAULT '',
		video_path TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		error TEXT NOT NULL DEFAULT '',
		task_id TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("建表失败: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Create(t *Task) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO tasks (prompt, image_path, status, created_at) VALUES (?,?,?,?)`,
		t.Prompt, t.ImagePath, string(StatusPending), time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateStatus(id int64, st Status, errMsg, videoPath, taskID string) error {
	var completed *time.Time
	if st == StatusDone || st == StatusFailed {
		now := time.Now()
		completed = &now
	}
	_, err := s.db.Exec(
		`UPDATE tasks SET status=?, error=?, video_path=?, task_id=?, completed_at=? WHERE id=?`,
		string(st), errMsg, videoPath, taskID, completed, id)
	return err
}

func (s *Store) ListAll() ([]Task, error) {
	rows, err := s.db.Query(`SELECT id, prompt, image_path, video_path, status, error, task_id, created_at, IFNULL(completed_at,'') FROM tasks ORDER BY id DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		var created, completed string
		if err := rows.Scan(&t.ID, &t.Prompt, &t.ImagePath, &t.VideoPath, &t.Status, &t.Error, &t.TaskID, &created, &completed); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		if completed != "" {
			ct, _ := time.Parse("2006-01-02 15:04:05", completed)
			t.CompletedAt = &ct
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) CountByStatus() (map[Status]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[Status]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		m[Status(st)] = n
	}
	return m, nil
}

func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id=?`, id)
	return err
}

// NextPending 原子取一个pending任务置为running（简易互斥：单写者下安全）。
func (s *Store) NextPending() (*Task, error) {
	var t Task
	var created string
	err := s.db.QueryRow(
		`SELECT id, prompt, image_path FROM tasks WHERE status='pending' ORDER BY id LIMIT 1`,
	).Scan(&t.ID, &t.Prompt, &t.ImagePath)
	if err != nil {
		return nil, err
	}
	_ = created
	if err := s.UpdateStatus(t.ID, StatusRunning, "", "", ""); err != nil {
		return nil, err
	}
	return &t, nil
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
