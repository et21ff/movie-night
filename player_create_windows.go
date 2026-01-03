// player_create_windows.go
//go:build windows

package main

import (
	"fmt"

	"movie-night/config"
	"movie-night/pkg/mpv"
)

func (app *App) createPlayer(cfg *config.Config, isController bool) error {
	fmt.Println("📺 [MPV] 使用 libmpv 模式")

	// 创建 libmpv 播放器
	player, err := mpv.NewLibMPVPlayer()
	if err != nil {
		return fmt.Errorf("创建播放器失败: %w", err)
	}

	// 加载视频
	if err := player.LoadFile(app.streamServer.GetURL()); err != nil {
		player.Close()
		return fmt.Errorf("加载视频失败: %w", err)
	}

	app.player = player
	app.monitor = player.GetMonitor()

	// 启动监控
	if app.monitor != nil {
		app.monitor.Start()
	}

	// 监听 MPV 窗口关闭
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		player.WaitForShutdown()

		app.mu.Lock()
		closed := app.closed
		app.mu.Unlock()

		if !closed {
			fmt.Println("\n📺 MPV 窗口已关闭")
			app.Shutdown()
		}
	}()

	return nil
}
