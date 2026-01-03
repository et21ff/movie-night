// main.go
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"movie-night/config"
	"movie-night/p2p"
	"movie-night/pkg/mpv"
	"movie-night/share"
	msync "movie-night/sync"
)

type App struct {
	p2pClient    *p2p.Client
	streamServer *p2p.StreamServer
	player       mpv.Player
	monitor      mpv.MonitorInterface
	mqttClient   *msync.MQTTClient

	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

func main() {
	var (
		isController bool
		shareCode    string
	)

	flag.BoolVar(&isController, "controller", false, "作为控制端运行")
	flag.StringVar(&shareCode, "join", "", "使用分享码加入房间")
	flag.Parse()

	// 有分享码 → 直接加入
	if shareCode != "" {
		joinWithCode(shareCode, isController)
		return
	}

	// 无参数 → 交互菜单
	interactiveMenu()
}

func interactiveMenu() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("🎬 P2P 视频同步播放器")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("  1. 创建房间（房主）")
	fmt.Println("  2. 加入房间（分享码）")
	fmt.Println("  0. 退出")
	fmt.Println()
	fmt.Print("请选择: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		createRoom()
	case "2":
		joinRoom()
	case "0":
		fmt.Println("👋 再见！")
	default:
		fmt.Println("❌ 无效选择")
	}
}

// 创建房间
func createRoom() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("📝 创建新房间")
	fmt.Println("━━━━━━━━━━━━━")
	fmt.Println()

	// 输入磁力链接
	fmt.Print("磁力链接: ")
	magnetLink, _ := reader.ReadString('\n')
	magnetLink = strings.TrimSpace(magnetLink)

	if !strings.HasPrefix(magnetLink, "magnet:") {
		fmt.Println("❌ 无效的磁力链接")
		return
	}

	// 输入或生成房间号
	fmt.Print("房间号 (留空自动生成): ")
	roomID, _ := reader.ReadString('\n')
	roomID = strings.TrimSpace(roomID)

	if roomID == "" {
		roomID = generateRoomID()
	}

	// 生成分享码
	code, err := share.Encode(magnetLink, roomID)
	if err != nil {
		fmt.Printf("❌ 生成分享码失败: %v\n", err)
		return
	}

	fmt.Println()
	fmt.Println("✅ 房间创建成功！")
	fmt.Println("━━━━━━━━━━━━━━━━")
	fmt.Printf("房间号: %s\n", roomID)
	fmt.Println()
	fmt.Println("📋 分享码（发给朋友）:")
	fmt.Println()
	fmt.Println(code)
	fmt.Println()

	// 确认启动
	fmt.Print("按 Enter 启动播放器...")
	reader.ReadString('\n')

	// 启动应用
	cfg := config.Default()
	cfg.MagnetLink = magnetLink
	cfg.MQTTTopic = "movie-night/" + roomID

	startApp(true, cfg)
}

// 加入房间
func joinRoom() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("🔗 加入房间")
	fmt.Println("━━━━━━━━━━━")
	fmt.Println()

	fmt.Print("输入分享码: ")
	code, _ := reader.ReadString('\n')
	code = strings.TrimSpace(code)

	if code == "" {
		fmt.Println("❌ 分享码不能为空")
		return
	}

	joinWithCode(code, false)
}

// 使用分享码加入
func joinWithCode(code string, isController bool) {
	info, err := share.Decode(code)
	if err != nil {
		fmt.Printf("❌ 无效的分享码: %v\n", err)
		return
	}

	role := "跟随端"
	if isController {
		role = "控制端"
	}

	fmt.Println()
	fmt.Println("🔗 解析成功")
	fmt.Println("━━━━━━━━━━")
	fmt.Printf("房间号: %s\n", info.RoomID)
	fmt.Printf("身份: %s\n", role)
	fmt.Println()

	cfg := config.Default()
	cfg.MagnetLink = info.MagnetLink
	cfg.MQTTTopic = "movie-night/" + info.RoomID

	startApp(isController, cfg)
}

// 生成随机房间号
func generateRoomID() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// 启动应用
func startApp(isController bool, cfg *config.Config) {
	role := "跟随端"
	if isController {
		role = "控制端"
	}
	fmt.Printf("🎬 启动播放器 (%s)\n\n", role)

	app := &App{}

	ctx, cancel := context.WithCancel(context.Background())
	app.cancel = cancel

	// 信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n📛 正在关闭...")
		app.Shutdown()
		os.Exit(0)
	}()

	defer app.Shutdown()

	if err := app.Run(ctx, isController, cfg); err != nil {
		log.Fatalf("❌ %v", err)
	}
}

// Run 运行应用
func (app *App) Run(ctx context.Context, isController bool, cfg *config.Config) error {
	var err error

	// 1. 启动 P2P
	app.p2pClient, err = p2p.NewClient(p2p.Config{
		DataDir:    cfg.DataDir,
		MaxConns:   cfg.MaxConns,
		MagnetLink: cfg.MagnetLink,
	})
	if err != nil {
		return fmt.Errorf("P2P 启动失败: %w", err)
	}

	// 2. 获取视频文件
	videoFile := app.p2pClient.GetLargestFile()
	if videoFile == nil {
		return fmt.Errorf("未找到视频文件")
	}
	fmt.Printf("📹 视频: %s\n\n", videoFile.DisplayPath())

	// 3. 启动流服务
	app.streamServer = p2p.NewStreamServer(cfg.StreamPort, videoFile)
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		app.streamServer.Start()
	}()
	time.Sleep(2 * time.Second)

	// 4. 创建播放器
	if err := app.createPlayer(cfg, isController); err != nil {
		return err
	}

	// 5. 获取时长
	time.Sleep(2 * time.Second)
	duration, _ := app.player.GetDuration()
	if duration > 0 {
		fmt.Printf("📹 时长: %.0f秒 (%.1f分钟)\n\n", duration, duration/60)
	}
	cfg.VideoDuration = duration

	// 6. 连接 MQTT
	app.mqttClient, err = msync.NewMQTTClient(msync.MQTTConfig{
		Broker:   cfg.MQTTBroker,
		ClientID: fmt.Sprintf("%s-%d", cfg.MQTTClientID, time.Now().Unix()),
		Topic:    cfg.MQTTTopic,
	})
	if err != nil {
		return fmt.Errorf("MQTT 连接失败: %w", err)
	}

	// 7. 同步逻辑
	if isController {
		controller := msync.NewController(
			app.mqttClient.GetClient(),
			app.mqttClient.GetTopic(),
			app.monitor.GetStatusChannel(),
			10*time.Second,
		)
		go controller.Start()
	} else {
		follower := msync.NewFollowerWithPlayer(app.player, app.mqttClient, cfg.VideoDuration)
		follower.Start()
	}

	// 8. 等待退出
	fmt.Println("⏳ 播放中，关闭窗口或 Ctrl+C 退出\n")
	<-ctx.Done()

	return nil
}

// Shutdown 关闭
func (app *App) Shutdown() {
	app.mu.Lock()
	if app.closed {
		app.mu.Unlock()
		return
	}
	app.closed = true
	app.mu.Unlock()

	if app.cancel != nil {
		app.cancel()
	}
	if app.monitor != nil {
		app.monitor.Stop()
	}
	if app.player != nil {
		app.player.Close()
	}
	if app.mqttClient != nil {
		app.mqttClient.Close()
	}
	if app.streamServer != nil {
		app.streamServer.Stop()
	}
	if app.p2pClient != nil {
		app.p2pClient.Close()
	}

	fmt.Println("✅ 已退出")
}

func getTitle(isController bool) string {
	if isController {
		return "P2P 同步播放器（控制端）"
	}
	return "P2P 同步播放器（跟随端）"
}
