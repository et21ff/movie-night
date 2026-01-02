package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"movie-night/config"
	"movie-night/p2p"
	"movie-night/pkg/mpv"
	"movie-night/sync"
)

func main() {
	// ===== 解析命令行参数 =====
	var isController bool
	flag.BoolVar(&isController, "controller", false, "作为控制端（房主）运行")
	flag.Parse()

	if isController {
		fmt.Println("🎬 运行模式: 控制端（房主）\n")
	} else {
		fmt.Println("🎬 运行模式: 跟随端（观众）\n")
	}

	// 1. 加载配置
	cfg := config.Default()

	// 2. 启动 P2P 客户端
	p2pClient, err := p2p.NewClient(p2p.Config{
		DataDir:    cfg.DataDir,
		MaxConns:   cfg.MaxConns,
		MagnetLink: cfg.MagnetLink,
	})
	if err != nil {
		log.Fatalf("❌ P2P 启动失败: %v", err)
	}
	defer p2pClient.Close()

	// 3. 获取视频文件
	videoFile := p2pClient.GetLargestFile()
	if videoFile == nil {
		log.Fatal("❌ 未找到视频文件")
	}
	fmt.Printf("📹 视频: %s\n\n", videoFile.DisplayPath())

	// 4. 启动 HTTP 流服务（后台）
	streamServer := p2p.NewStreamServer(cfg.StreamPort, videoFile)
	go func() {
		if err := streamServer.Start(); err != nil {
			log.Fatal(err)
		}
	}()

	// 5. 启动 MPV（后台）
	go func() {
		if err := mpv.Launch(mpv.LaunchConfig{
			VideoURL:   streamServer.GetURL(),
			SocketPath: cfg.MPVSocketPath,
			Title:      getTitle(isController),
		}); err != nil {
			log.Printf("MPV 退出: %v", err)
		}
		os.Exit(0)
	}()

	// 6. 等待 MPV 启动
	time.Sleep(2 * time.Second)

	// 7. 创建 MPV 控制器
	mpvCtrl, err := mpv.NewController(cfg.MPVSocketPath)
	if err != nil {
		log.Fatalf("❌ MPV 控制器创建失败: %v", err)
	}
	defer mpvCtrl.Close()

	// ===== 8. 创建 MPV 监听器（监听播放状态）=====
	monitor, err := mpv.NewMonitor(cfg.MPVSocketPath)
	if err != nil {
		log.Fatalf("❌ 创建监听器失败: %v", err)
	}
	defer monitor.Stop()
	monitor.Start()

	// 9. 获取视频时长
	time.Sleep(1 * time.Second)
	duration, err := mpvCtrl.GetDuration()
	if err != nil {
		log.Printf("⚠️  无法获取视频时长: %v", err)
		duration = 0
	} else {
		fmt.Printf("📹 时长: %.0f秒 (%.1f分钟)\n\n", duration, duration/60)
	}
	cfg.VideoDuration = duration

	// 10. 连接 MQTT
	mqttClient, err := sync.NewMQTTClient(sync.MQTTConfig{
		Broker:   cfg.MQTTBroker,
		ClientID: fmt.Sprintf("%s-%d", cfg.MQTTClientID, time.Now().Unix()),
		Topic:    cfg.MQTTTopic,
	})
	if err != nil {
		log.Fatalf("❌ MQTT 连接失败: %v", err)
	}
	defer mqttClient.Close()

	// ===== 11. 根据角色启动不同逻辑 =====
	if isController {
		// ===== 传入原始 client 和 topic =====
		// 需要修改 NewMQTTClient 返回原始 client
		// 或者创建一个 GetClient() 方法

		// 方式 A：修改 MQTTClient 添加 GetClient 方法
		controller := sync.NewController(
			mqttClient.GetClient(), // 获取原始 client
			cfg.MQTTTopic,
			monitor,
			10*time.Second,
		)
		go controller.Start()
	} else {
		follower := sync.NewFollower(mpvCtrl, mqttClient, cfg.VideoDuration)
		follower.Start()
	}

	// 12. 启动 P2P 统计推送
	statsPusher := p2p.NewStatsPusher(p2pClient.GetTorrent(), cfg.MPVSocketPath)
	go func() {
		if err := statsPusher.Start(); err != nil {
			log.Printf("⚠️  统计推送失败: %v", err)
		}
	}()

	// 13. 保持运行
	fmt.Println("⏳ 运行中，按 Ctrl+C 退出\n")
	select {}
}

// getTitle 获取窗口标题
func getTitle(isController bool) string {
	if isController {
		return "P2P 同步播放器（控制端）"
	}
	return "P2P 同步播放器（跟随端）"
}
