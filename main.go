package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"movie-night/config"
	"movie-night/pkg/mpv"
	"movie-night/sync"

	"github.com/anacrolix/torrent"
)

// 测试磁力链 (Sintel)
const MagnetLink = "magnet:?xt=urn:btih:JEJJEE6LGDVRMHT7XVJGJ74BKVW6WL2M&dn=&tr=http%3A%2F%2F104.143.10.186%3A8000%2Fannounce&tr=udp%3A%2F%2F104.143.10.186%3A8000%2Fannounce&tr=http%3A%2F%2Ftracker.openbittorrent.com%3A80%2Fannounce&tr=http%3A%2F%2Ftracker3.itzmx.com%3A6961%2Fannounce&tr=http%3A%2F%2Ftracker4.itzmx.com%3A2710%2Fannounce&tr=http%3A%2F%2Ftracker.publicbt.com%3A80%2Fannounce&tr=http%3A%2F%2Ftracker.prq.to%2Fannounce&tr=http%3A%2F%2Fopen.acgtracker.com%3A1096%2Fannounce&tr=https%3A%2F%2Ft-115.rhcloud.com%2Fonly_for_ylbud&tr=http%3A%2F%2Ftracker1.itzmx.com%3A8080%2Fannounce&tr=http%3A%2F%2Ftracker2.itzmx.com%3A6961%2Fannounce&tr=udp%3A%2F%2Ftracker1.itzmx.com%3A8080%2Fannounce&tr=udp%3A%2F%2Ftracker2.itzmx.com%3A6961%2Fannounce&tr=udp%3A%2F%2Ftracker3.itzmx.com%3A6961%2Fannounce&tr=udp%3A%2F%2Ftracker4.itzmx.com%3A2710%2Fannounce&tr=http%3A%2F%2Ftr.bangumi.moe%3A6969%2Fannounce"

// IPC Socket 路径 (Linux/macOS 通常在 /tmp, Windows 是命名管道)
var socketPath = filepath.Join(os.TempDir(), "mpv-socket")

// JSON 输出结构 (对应你之前的 bash+jq 脚本)
type FileEntry struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Size  string `json:"size"`
	URL   string `json:"url"`
}

// MPV 发回来的消息结构
type MPVEvent struct {
	Event string      `json:"event"`
	Name  string      `json:"name"`
	Data  interface{} `json:"data"` // Data 可能是数字(时间)也可能是布尔(暂停)
	Error string      `json:"error"`
}

var (
	isController  bool
	mqttClient    *sync.MQTTClient
	syncer        *sync.Syncer
	mpvController *mpv.Controller
)

func main() {
	// 1. 启动 P2P 引擎
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = "./downloads"
	cfg.EstablishedConnsPerTorrent = 50
	cfg.DisableAggressiveUpload = true
	// cfg.Debug = true // 调试时可以打开

	fmt.Println("🚀 [Core] 正在启动 P2P 引擎...")
	client, err := torrent.NewClient(cfg)
	if err != nil {
		log.Fatalf("创建 Client 失败: %v", err)
	}
	defer client.Close()

	// 2. 添加磁力链
	t, err := client.AddMagnet(MagnetLink)
	if err != nil {
		log.Fatalf("添加磁力链失败: %v", err)
	}

	fmt.Println("🔍 [Core] 正在寻找 Peers 获取元数据...")
	<-t.GotInfo() // 阻塞直到拿到文件列表

	// 3. 选出最大的文件 (视频)
	files := t.Files()
	sort.Slice(files, func(i, j int) bool {
		return files[i].Length() > files[j].Length()
	})
	targetFile := files[0]

	// 4. (复刻你的 jq 脚本) 打印文件列表 JSON
	// 这部分虽然 MPV 不直接用，但你的“主控程序”未来可能需要这个列表来选集
	printJSONList(files)

	// 5. 启动 HTTP Server
	go func() {
		http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
			// 关键优化：响应式读取，优先下载请求的块
			reader := targetFile.NewReader()
			reader.SetResponsive()
			defer reader.Close()
			http.ServeContent(w, r, targetFile.DisplayPath(), time.Now(), reader)
		})
		if err := http.ListenAndServe(":8888", nil); err != nil {
			log.Fatal(err)
		}
	}()
	fmt.Println("📡 [HTTP] 流媒体服务运行在 http://localhost:8888/stream")

	// 6. 启动 MPV
	go startMPV("http://localhost:8888/stream")
	go monitorMPV(t)

	// 7. (复刻你的 awk 脚本) 实时推送状态到 MPV
	// 等待 MPV 启动并创建 Socket
	time.Sleep(2 * time.Second)

	if err := initMQTTFollower(); err != nil {
		log.Printf("⚠️  MQTT 初始化失败: %v", err)
		log.Println("继续运行，但无同步功能")
	}

	pushStatsToMPV(t)
}
func initMQTTFollower() error {
	fmt.Println("📡 初始化 MQTT 同步...\n")

	// 1. 加载配置
	cfg := config.Default()

	// 2. 创建 MQTT 客户端
	mqttClient, err := sync.NewMQTTClient(sync.MQTTConfig{
		Broker:   cfg.MQTTBroker,
		ClientID: fmt.Sprintf("video-follower-%d", time.Now().Unix()),
		Topic:    cfg.MQTTTopic,
	})
	if err != nil {
		return fmt.Errorf("MQTT 连接失败: %w", err)
	}

	// 3. 创建 MPV 控制器
	mpvController, err := mpv.NewController(socketPath)
	if err != nil {
		return fmt.Errorf("MPV 控制器创建失败: %w", err)
	}

	// 4. 创建同步器
	syncer := sync.NewSyncer(mpvController, cfg.VideoDuration)
	syncer.Start()

	// 5. 订阅 MQTT
	if err := mqttClient.Subscribe(syncer.HandleStatus); err != nil {
		return fmt.Errorf("订阅失败: %w", err)
	}

	fmt.Println("✅ MQTT 同步已启动")
	fmt.Println("📺 等待控制命令...\n")

	return nil
}

// startMPV 启动前端播放器
func startMPV(url string) {
	// 如果 socket 文件已存在，先删除，防止连接错误
	if _, err := os.Stat(socketPath); err == nil {
		os.Remove(socketPath)
	}

	args := []string{
		url,
		"--input-ipc-server=" + socketPath, // 开启 IPC
		"--force-window",
		"--title=Movie Night (P2P)",
		// "--fs", // 全屏
	}

	fmt.Printf("📺 [MPV] 启动播放器... (IPC: %s)\n", socketPath)
	cmd := exec.Command("mpv", args...)
	cmd.Stdout = os.Stdout // 把 MPV 的日志接管过来
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("MPV 退出: %v", err)
	}
	// MPV 关闭后，主程序也退出
	os.Exit(0)
}

func monitorMPV(t *torrent.Torrent) {
	// 1. 连接 Socket (复用之前的逻辑)
	var conn net.Conn
	var err error
	for i := 0; i < 10; i++ {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return
	}
	defer conn.Close()

	// 2. 发送监听指令
	// 告诉 MPV: "我要监听 'time-pos' (时间) 和 'pause' (暂停状态)"
	// 参数 1 是观察者 ID，随便填
	cmds := []string{
		`{ "command": ["observe_property", 1, "time-pos"] }`,
		`{ "command": ["observe_property", 2, "pause"] }`,
	}
	for _, cmd := range cmds {
		conn.Write([]byte(cmd + "\n"))
	}

	fmt.Println("👂 [Monitor] 已开始监听 MPV 播放进度...")

	// 3. 开启读取循环 (Reader)
	decoder := json.NewDecoder(conn)
	for {
		var event MPVEvent
		// 阻塞读取，直到 MPV 发来新消息
		if err := decoder.Decode(&event); err != nil {
			log.Printf("MPV 连接断开: %v", err)
			return
		}

		// 处理事件
		if event.Event == "property-change" {
			switch event.Name {
			case "time-pos":
				// data 可能是 float64
				if seconds, ok := event.Data.(float64); ok {
					// 【这里就是你要的数据！】
					// 可以在这里把 seconds 发送到 P2P 网络进行同步
					fmt.Printf("\r>> 前端播放进度: %.2f 秒  ", seconds)
				}
			case "pause":
				if isPaused, ok := event.Data.(bool); ok {
					status := "播放中"
					if isPaused {
						status = "已暂停"
					}
					fmt.Printf("\n>> 前端状态变更: %s\n", status)
				}
			}
		}
	}
}

func pushStatsToMPV(t *torrent.Torrent) {
	// 1. 尝试连接 IPC (和之前一样)
	var conn net.Conn
	var err error

	for i := 0; i < 10; i++ {
		if runtime.GOOS == "windows" {
			log.Println("Windows IPC 需要额外配置，跳过 OSD 推送。")
			return
		}
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		log.Printf("⚠️ 无法连接 MPV IPC, 仪表盘功能失效: %v", err)
		return
	}
	defer conn.Close()

	fmt.Println("🔌 [IPC] 已连接 MPV，开始推送 OSD 数据")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 2. 初始化用于计算速度的变量
	var lastBytes int64 = 0

	for range ticker.C {
		stats := t.Stats()

		// --- 修复点开始 ---
		// 使用 BytesRead (总读取字节) 替代 BytesReadUseful
		currentBytes := stats.ConnStats.BytesRead.Int64()

		// 计算这一秒内的增量 (即速度)
		// 如果是第一次循环，速度设为 0，防止数据突变
		var speedBytes int64 = 0
		if lastBytes != 0 {
			speedBytes = currentBytes - lastBytes
		}
		lastBytes = currentBytes

		// 转换为 MB/s
		speedMB := float64(speedBytes) / 1024 / 1024

		// 计算进度百分比 (已完成字节 / 总字节)
		// 注意：TotalLength() 可能在元数据没取到前是 0
		var progress float64 = 0
		if t.Length() > 0 {
			progress = float64(t.BytesCompleted()) / float64(t.Length()) * 100
		}
		// --- 修复点结束 ---

		// 构造显示文本
		msg := fmt.Sprintf("P2P 速度: %.2f MB/s | 进度: %.1f%% | Peers: %d",
			speedMB,
			progress,
			len(t.PeerConns()),
		)

		// 发送给 MPV
		cmd := map[string]interface{}{
			"command": []interface{}{"show-text", msg, 1000},
		}

		jsonBytes, _ := json.Marshal(cmd)
		conn.Write(jsonBytes)
		conn.Write([]byte("\n"))
	}
}

// 辅助函数：格式化文件大小
func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// printJSONList 模拟你的 jq 脚本输出
func printJSONList(files []*torrent.File) {
	var list []FileEntry
	for i, f := range files {
		list = append(list, FileEntry{
			Index: i,
			Name:  f.DisplayPath(),
			Size:  formatSize(f.Length()),
			URL:   "http://localhost:8888/stream", // 简化处理，暂时都指向同一个流
		})
	}

	jsonData, _ := json.MarshalIndent(list, "", "  ")
	fmt.Println("\n--- File List (JSON) ---")
	fmt.Println(string(jsonData))
	fmt.Println("------------------------\n")
}
