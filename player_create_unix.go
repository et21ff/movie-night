// player_create_unix.go
//go:build !windows

package main

import (
	"fmt"
	"time"

	"movie-night/config"
	"movie-night/pkg/mpv"
)

func (app *App) createPlayer(cfg *config.Config, isController bool) error {
	fmt.Println("📺 [MPV] 使用 IPC 模式")

	// 启动 mpv 进程
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()

		err := mpv.Launch(mpv.LaunchConfig{
			VideoURL:   app.streamServer.GetURL(),
			SocketPath: cfg.MPVSocketPath,
			Title:      getTitle(isController),
		})

		app.mu.Lock()
		closed := app.closed
		app.mu.Unlock()

		if err != nil && !closed {
			fmt.Printf("❌ MPV 退出: %v\n", err)
		}

		if !closed {
			fmt.Println("\n📺 MPV 进程已退出")
			app.Shutdown()
		}
	}()

	// 等待 MPV 启动
	time.Sleep(2 * time.Second)

	// 连接 IPC
	player, err := mpv.NewIPCPlayer(cfg.MPVSocketPath)
	if err != nil {
		return fmt.Errorf("连接 MPV 失败: %w", err)
	}

	app.player = player
	app.monitor = player.GetMonitor()

	// 启动监控
	if app.monitor != nil {
		app.monitor.Start()
	}

	return nil
}
