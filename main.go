package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"movie-night/config"
	"movie-night/p2p"
	"movie-night/pkg/mpv"
	msync "movie-night/sync" // 重命名避免与 sync 包冲突
)

// App 应用程序状态，用于资源管理
type App struct {
	p2pClient    *p2p.Client
	streamServer *p2p.StreamServer
	player       mpv.Player
	monitor      mpv.MonitorInterface
	mqttClient   *msync.MQTTClient
	statsPusher  *p2p.StatsPusher

	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

func main() {
	// 1. 解析命令行参数
	var isController bool
	flag.BoolVar(&isController, "controller", false, "作为控制端（房主）运行")
	flag.Parse()

	role := "跟随端"
	if isController {
		role = "控制端"
	}
	fmt.Printf("🎬 P2P 视频同步播放器 (%s)\n\n", role)

	// 2. 创建应用实例
	app := &App{}

	// ✅ 设置信号处理（Ctrl+C）
	ctx, cancel := context.WithCancel(context.Background())
	app.cancel = cancel

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// ✅ 信号处理 goroutine
	go func() {
		sig := <-sigChan
		fmt.Printf("\n\n📛 收到信号: %v，正在关闭...\n", sig)
		app.Shutdown()
		os.Exit(0)
	}()

	// ✅ 确保退出时清理
	defer app.Shutdown()

	// 3. 运行应用
	if err := app.Run(ctx, isController); err != nil {
		log.Fatalf("❌ %v", err)
	}
}

func (app *App) Run(ctx context.Context, isController bool) error {
	// 1. 加载配置
	cfg := config.Default()

	// 2. 启动 P2P 客户端
	var err error
	app.p2pClient, err = p2p.NewClient(p2p.Config{
		DataDir:    cfg.DataDir,
		MaxConns:   cfg.MaxConns,
		MagnetLink: cfg.MagnetLink,
	})
	if err != nil {
		return fmt.Errorf("P2P 启动失败: %w", err)
	}

	// 3. 获取视频文件
	videoFile := app.p2pClient.GetLargestFile()
	if videoFile == nil {
		return fmt.Errorf("未找到视频文件")
	}
	fmt.Printf("📹 视频: %s\n\n", videoFile.DisplayPath())

	// 4. 启动 HTTP 流服务
	app.streamServer = p2p.NewStreamServer(cfg.StreamPort, videoFile)
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		fmt.Println("🔧 [DEBUG] 启动流服务 goroutine...")
		if err := app.streamServer.Start(); err != nil {
			if !app.closed {
				fmt.Printf("❌ HTTP 流服务错误: %v\n", err)
			}
		}
	}()
	time.Sleep(2 * time.Second)

	// 5. 创建播放器
	if err := app.createPlayer(cfg, isController); err != nil {
		return err
	}

	// 6. 获取视频时长
	time.Sleep(2 * time.Second)
	duration, err := app.player.GetDuration()
	if err != nil {
		log.Printf("⚠️  无法获取视频时长: %v", err)
		duration = 0
	} else {
		fmt.Printf("📹 时长: %.0f秒 (%.1f分钟)\n\n", duration, duration/60)
	}
	cfg.VideoDuration = duration

	// 7. 连接 MQTT
	app.mqttClient, err = msync.NewMQTTClient(msync.MQTTConfig{
		Broker:   cfg.MQTTBroker,
		ClientID: fmt.Sprintf("%s-%d", cfg.MQTTClientID, time.Now().Unix()),
		Topic:    cfg.MQTTTopic,
	})
	if err != nil {
		return fmt.Errorf("MQTT 连接失败: %w", err)
	}

	// 8. 启动同步逻辑
	if isController {
		controller := msync.NewController(
			app.mqttClient.GetClient(),
			app.mqttClient.GetTopic(),
			app.monitor.GetStatusChannel(),
			10*time.Second,
		)
		app.wg.Add(1)
		go func() {
			defer app.wg.Done()
			controller.Start()
		}()
	} else {
		follower := msync.NewFollowerWithPlayer(app.player, app.mqttClient, cfg.VideoDuration)
		if err := follower.Start(); err != nil {
			return fmt.Errorf("跟随端启动失败: %w", err)
		}
	}

	// 9. 启动 P2P 统计推送
	app.statsPusher = p2p.NewStatsPusher(app.p2pClient.GetTorrent(), cfg.MPVSocketPath)
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		if err := app.statsPusher.Start(); err != nil {
			if !app.closed {
				log.Printf("⚠️  统计推送失败: %v", err)
			}
		}
	}()

	// ✅ 10. 等待退出（统一处理）
	fmt.Println("\n⏳ 播放中，关闭 MPV 窗口或按 Ctrl+C 退出\n")
	<-ctx.Done()

	fmt.Println("\n👋 程序退出")
	return nil
}

func (app *App) createPlayer(cfg *config.Config, isController bool) error {
	if runtime.GOOS == "windows" {
		// === Windows: libmpv 模式 ===
		fmt.Println("📺 [MPV] 使用 libmpv 模式")

		libPlayer, err := mpv.NewLibMPVPlayer()
		if err != nil {
			return fmt.Errorf("创建播放器失败: %w", err)
		}

		if err := libPlayer.LoadFile(app.streamServer.GetURL()); err != nil {
			libPlayer.Close()
			return fmt.Errorf("加载视频失败: %w", err)
		}

		app.player = libPlayer
		app.monitor = libPlayer.GetMonitor()
		if app.monitor != nil {
			app.monitor.Start()
		}

		// ✅ 监听 MPV 窗口关闭
		app.wg.Add(1)
		go func() {
			defer app.wg.Done()

			// 阻塞直到 MPV 关闭
			libPlayer.WaitForShutdown()

			// 触发应用退出
			app.mu.Lock()
			closed := app.closed
			app.mu.Unlock()

			if !closed {
				fmt.Println("\n📺 MPV 窗口已关闭，正在退出...")
				app.Shutdown()
			}
		}()

	} else {
		// === Unix: IPC 模式 ===
		fmt.Println("📺 [MPV] 使用 IPC 模式")

		// 启动 MPV 进程
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
				log.Printf("MPV 退出: %v", err)
			}

			if !closed {
				fmt.Println("\n📺 MPV 进程已退出，正在退出...")
				app.Shutdown()
			}
		}()

		// 等待 MPV 启动
		time.Sleep(2 * time.Second)

		// 连接 IPC
		ipcPlayer, err := mpv.NewIPCPlayer(cfg.MPVSocketPath)
		if err != nil {
			return fmt.Errorf("创建播放器失败: %w", err)
		}

		app.player = ipcPlayer
		app.monitor = ipcPlayer.GetMonitor()
		if app.monitor != nil {
			app.monitor.Start()
		}
	}

	return nil
}

// ✅ Shutdown 优雅关闭所有资源
func (app *App) Shutdown() {
	app.mu.Lock()
	if app.closed {
		app.mu.Unlock()
		return
	}
	app.closed = true
	app.mu.Unlock()

	fmt.Println("🛑 正在关闭所有服务...")

	// 取消 context
	if app.cancel != nil {
		app.cancel()
	}

	// 1. 停止监控
	if app.monitor != nil {
		fmt.Println("  🔧 停止状态监控...")
		app.monitor.Stop()
	}

	// 2. 关闭播放器
	if app.player != nil {
		fmt.Println("  🔧 关闭播放器...")
		app.player.Close()
	}

	// 3. 停止统计推送
	if app.statsPusher != nil {
		fmt.Println("  🔧 停止统计推送...")
		app.statsPusher.Stop()
	}

	// 4. 关闭 MQTT
	if app.mqttClient != nil {
		fmt.Println("  🔧 关闭 MQTT 连接...")
		app.mqttClient.Close()
	}

	// 5. 停止 HTTP 流服务
	if app.streamServer != nil {
		fmt.Println("  🔧 关闭 HTTP 流服务...")
		app.streamServer.Stop()
	}

	// 6. 关闭 P2P 客户端
	if app.p2pClient != nil {
		fmt.Println("  🔧 关闭 P2P 连接...")
		app.p2pClient.Close()
	}

	// 等待所有 goroutine 完成（最多 3 秒）
	done := make(chan struct{})
	go func() {
		app.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("✅ 所有服务已安全关闭")
	case <-time.After(3 * time.Second):
		fmt.Println("⚠️  部分服务关闭超时，强制退出")
	}
}

func getTitle(isController bool) string {
	if isController {
		return "P2P 同步播放器（控制端）"
	}
	return "P2P 同步播放器（跟随端）"
}
