// pkg/mpv/factory_unix.go
//go:build !windows

package mpv

import (
	"fmt"
	"time"
)

// NewPlayer 创建播放器（Unix: IPC）
func NewPlayer() (Player, error) {
	fmt.Println("📺 [MPV] 使用 IPC 模式")
	// 返回一个需要后续设置的 IPC 播放器
	return nil, fmt.Errorf("请使用 NewIPCPlayer")
}

// NewPlayerWithConfig 使用配置创建播放器
func NewPlayerWithConfig(socketPath string, videoURL string, title string) (Player, MonitorInterface, error) {
	fmt.Println("📺 [MPV] 使用 IPC 模式")

	// 启动 mpv 进程
	go func() {
		Launch(LaunchConfig{
			VideoURL:   videoURL,
			SocketPath: socketPath,
			Title:      title,
		})
	}()

	// 等待启动
	time.Sleep(2 * time.Second)

	// 连接 IPC
	player, err := NewIPCPlayer(socketPath)
	if err != nil {
		return nil, nil, err
	}

	return player, player.GetMonitor(), nil
}

// NewMonitorFromPlayer 从播放器获取监控器
func NewMonitorFromPlayer(p Player) MonitorInterface {
	if ip, ok := p.(*IPCPlayer); ok {
		return ip.GetMonitor()
	}
	return nil
}
