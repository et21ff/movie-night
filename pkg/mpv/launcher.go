package mpv

import (
	"fmt"
	"os"
	"os/exec"
)

// LaunchConfig MPV 启动配置
type LaunchConfig struct {
	VideoURL   string
	SocketPath string
	Title      string
	Fullscreen bool
}

// Launch 启动 MPV 播放器（阻塞）
func Launch(cfg LaunchConfig) error {
	// 删除旧 Socket
	if _, err := os.Stat(cfg.SocketPath); err == nil {
		os.Remove(cfg.SocketPath)
	}

	args := []string{
		cfg.VideoURL,
		"--input-ipc-server=" + cfg.SocketPath,
		"--force-window",
		"--title=" + cfg.Title,
	}

	if cfg.Fullscreen {
		args = append(args, "--fs")
	}

	fmt.Printf("📺 [MPV] 启动播放器\n")
	fmt.Printf("   视频: %s\n", cfg.VideoURL)
	fmt.Printf("   Socket: %s\n", cfg.SocketPath)

	cmd := exec.Command("mpv", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
