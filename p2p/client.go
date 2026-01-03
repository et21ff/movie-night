package p2p

import (
	"fmt"
	"sort"

	"github.com/anacrolix/torrent"
)

// Client P2P 客户端
type Client struct {
	client  *torrent.Client
	torrent *torrent.Torrent
}

// Config P2P 配置
type Config struct {
	DataDir    string
	MaxConns   int
	MagnetLink string
}

// NewClient 创建 P2P 客户端
func NewClient(cfg Config) (*Client, error) {
	// 配置 torrent 客户端
	tcfg := torrent.NewDefaultClientConfig()
	tcfg.DataDir = cfg.DataDir
	tcfg.EstablishedConnsPerTorrent = cfg.MaxConns
	tcfg.DisableAggressiveUpload = true

	fmt.Println("🚀 [P2P] 启动引擎...")
	client, err := torrent.NewClient(tcfg)
	if err != nil {
		return nil, fmt.Errorf("创建客户端失败: %w", err)
	}

	// 添加磁力链
	t, err := client.AddMagnet(cfg.MagnetLink)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("添加磁力链失败: %w", err)
	}
	trackers := []string{
		"udp://tracker.opentrackr.org:6969/announce",
		"udp://tracker.openbittorrent.com:6969/announce",
		"udp://tracker1.bt.krim.net:6969/announce",
	}

	for _, trackerURL := range trackers {
		t.AddTrackers([][]string{{trackerURL}})
		fmt.Printf("✅ [P2P] 已添加 Tracker: %s\n", trackerURL)
	}

	fmt.Println("🔍 [P2P] 获取元数据...")
	<-t.GotInfo()

	return &Client{
		client:  client,
		torrent: t,
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

// Close 关闭客户端
func (c *Client) Close() error {
	c.client.Close()
	return nil
}
