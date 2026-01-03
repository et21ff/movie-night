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
	dataDir      string
	cleanOnClose bool
}

// Config P2P 配置
type Config struct {
	DataDir      string
	MaxConns     int
	MagnetLink   string
	CleanOnClose bool
}

func NewClient(cfg Config) (*Client, error) {
	// 配置 torrent 客户端
	tcfg := torrent.NewDefaultClientConfig()

	tcfg.DataDir = cfg.DataDir
	tcfg.EstablishedConnsPerTorrent = 80
	tcfg.HalfOpenConnsPerTorrent = 40
	tcfg.TotalHalfOpenConns = 100

	// 功能开关
	tcfg.Seed = true
	tcfg.NoDHT = false
	tcfg.DisablePEX = false
	tcfg.DisableUTP = false
	tcfg.DisableTCP = false
	tcfg.DisableIPv6 = false
	tcfg.DisableAcceptRateLimiting = true

	// 速度设置
	tcfg.DownloadRateLimiter = rate.NewLimiter(rate.Inf, 0)
	tcfg.UploadRateLimiter = rate.NewLimiter(rate.Inf, 0)

	fmt.Println("🚀 [P2P] 启动引擎...")
	client, err := torrent.NewClient(tcfg)
	if err != nil {
		return nil, fmt.Errorf("创建客户端失败: %w", err)
	}

	// Trackers
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

	// ✅ 改动 1: 添加这一行，开始下载
	t.DownloadAll()
	fmt.Println("📥 [P2P] 开始下载...")

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

	largest := files[0]

	// ✅ 改动 2: 添加这一行，确保文件被标记下载
	largest.Download()

	return largest
}

// GetTorrent 获取原始 Torrent 对象
func (c *Client) GetTorrent() *torrent.Torrent {
	return c.torrent
}

// Close 关闭客户端
func (c *Client) Close() error {
	c.client.Close()
	if c.cleanOnClose {
		if err := emptyDir(c.dataDir); err != nil {
			return fmt.Errorf("清理缓存失败: %w", err)
		}
	}
	return nil
}

// 清空目录内容
func emptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
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
