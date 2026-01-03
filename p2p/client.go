package p2p

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/anacrolix/torrent"
	"golang.org/x/time/rate"
)

// Client P2P 客户端
type Client struct {
	client       *torrent.Client
	torrent      *torrent.Torrent
	dataDir      string // 新增：记录当前客户端使用的缓存目录
	cleanOnClose bool   // 新增：关闭时是否清理缓存
}

// Config P2P 配置
type Config struct {
	DataDir      string
	MaxConns     int
	MagnetLink   string
	CleanOnClose bool // 新增：关闭时清空缓存目录
}

func NewClient(cfg Config) (*Client, error) {
	// 配置 torrent 客户端
	tcfg := torrent.NewDefaultClientConfig()

	tcfg.DataDir = cfg.DataDir
	tcfg.EstablishedConnsPerTorrent = cfg.MaxConns
	tcfg.DisableAggressiveUpload = true
	tcfg.EstablishedConnsPerTorrent = 80 // 每个种子最大连接数
	tcfg.HalfOpenConnsPerTorrent = 40    // 半开连接数
	tcfg.TotalHalfOpenConns = 100        // 总半开连接数
	// ========== 功能开关 ==========
	tcfg.Seed = true // ✅ 做种，有助于获取更多 peers
	// tcfg.NoDHT = false                    // ✅ 启用 DHT
	// tcfg.DisablePEX = false               // ✅ 启用 PEX（Peer Exchange）
	// tcfg.DisableUTP = false               // ✅ 启用 uTP
	// tcfg.DisableTCP = false               // ✅ 启用 TCP
	// tcfg.DisableIPv6 = false              // ✅ 启用 IPv6
	// tcfg.DisableAcceptRateLimiting = true // ✅ 禁用连接速率限制

	// ========== 速度设置 ==========
	tcfg.DownloadRateLimiter = rate.NewLimiter(rate.Inf, 0) // 无限下载速度
	tcfg.UploadRateLimiter = rate.NewLimiter(rate.Inf, 0)   // 无限上传速度

	fmt.Println("🚀 [P2P] 启动引擎...")
	client, err := torrent.NewClient(tcfg)
	if err != nil {
		return nil, fmt.Errorf("创建客户端失败: %w", err)
	}

	// 写死 trackers 并拼接到磁力链
	magnet := cfg.MagnetLink
	trackers := []string{
		"udp://tracker.opentrackr.org:1337/announce",
		"udp://tracker.openbittorrent.com:6969/announce",
		"udp://tracker.internetwarriors.net:1337/announce",
		"http://tracker.opentrackr.org:1337/announce",
	}
	u, err := url.Parse(magnet)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("磁力链解析失败: %w", err)
	}
	q := u.Query()
	for _, tr := range trackers {
		q.Add("tr", tr)
	}
	u.RawQuery = q.Encode()
	magnet = u.String()
	fmt.Printf("🧭 [P2P] 添加 %d 个 Tracker\n", len(trackers))

	// 添加磁力链
	t, err := client.AddMagnet(magnet)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("添加磁力链失败: %w", err)
	}

	fmt.Println("🔍 [P2P] 获取元数据...")
	<-t.GotInfo()

	return &Client{
		client:       client,
		torrent:      t,
		dataDir:      cfg.DataDir,
		cleanOnClose: cfg.CleanOnClose,
	}, nil
}

// GetLargestFile 获取最大的文件（视频）
func (c *Client) GetLargestFile() *torrent.File {
	files := c.torrent.Files()
	if len(files) == 0 {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Length() > files[j].Length()
	})

	return files[0]
}

// GetTorrent 获取原始 Torrent 对象（用于统计）
func (c *Client) GetTorrent() *torrent.Torrent {
	return c.torrent
}

// Close 关闭客户端（可选清空缓存目录）
func (c *Client) Close() error {
	c.client.Close()
	if c.cleanOnClose {
		if err := emptyDir(c.dataDir); err != nil {
			return fmt.Errorf("清理缓存失败: %w", err)
		}
	}
	return nil
}

// 清空目录内容，但保留目录本身
func emptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// 目录不存在则视为已清空
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}
