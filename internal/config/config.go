// Package config 环境变量配置。密钥不入库、不落盘。
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	ListenAddr string
	DBPath     string
	DataDir    string // 上传图片和生成视频的存储目录
	APIKey     string // MiniMax API Key
	VideoModel string
	MaxWorkers int    // 并发生成数
	DryRun     bool   // 模拟模式：不调API，不烧额度
}

func FromEnv() (*Config, error) {
	c := &Config{
		ListenAddr: env("DS_LISTEN", ":9090"),
		DBPath:     env("DS_DB", "data/dramastudio.db"),
		DataDir:    env("DS_DATA", "data"),
		APIKey:     os.Getenv("DS_API_KEY"),
		VideoModel: env("DS_VIDEO_MODEL", "MiniMax-Hailuo-2.3"),
		MaxWorkers: 2,
		DryRun:     os.Getenv("DS_DRY_RUN") == "true",
	}
	if c.MaxWorkers < 1 {
		c.MaxWorkers = 1
	}
	if c.MaxWorkers > 5 {
		c.MaxWorkers = 5
	}
	if c.APIKey == "" && !c.DryRun {
		return nil, fmt.Errorf("DS_API_KEY 未设置（或设 DS_DRY_RUN=true 用模拟模式体验）")
	}
	return c, nil
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
