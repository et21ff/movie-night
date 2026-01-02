package p2p

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/anacrolix/torrent"
	"movie-night/pkg/mpv"
)

// StatsPusher 统计推送器
type StatsPusher struct {
	torrent    *torrent.Torrent
	socketPath string
}

// NewStatsPusher 创建统计推送器
func NewStatsPusher(t *torrent.Torrent, socketPath string) *StatsPusher {
	return &StatsPusher{
		torrent:    t,
		socketPath: socketPath,
	}
}

// Start 启动推送（阻塞）
func (s *StatsPusher) Start() error {
	var conn net.Conn
	var err error

	// 连接 MPV IPC
	for i := 0; i < 10; i++ {
		conn, err = mpv.DialSocket(s.socketPath)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		return fmt.Errorf("连接 MPV IPC 失败: %w", err)
	}
	defer conn.Close()

	fmt.Println("🔌 [IPC] 已连接 MPV，推送 P2P 统计")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastBytes int64

	for range ticker.C {
		stats := s.torrent.Stats()
		currentBytes := stats.ConnStats.BytesRead.Int64()

		var speedBytes int64
		if lastBytes != 0 {
			speedBytes = currentBytes - lastBytes
		}
		lastBytes = currentBytes

		speedMB := float64(speedBytes) / 1024 / 1024

		var progress float64
		if s.torrent.Length() > 0 {
			progress = float64(s.torrent.BytesCompleted()) / float64(s.torrent.Length()) * 100
		}

		msg := fmt.Sprintf("P2P: %.2f MB/s | %.1f%% | %d peers",
			speedMB,
			progress,
			len(s.torrent.PeerConns()),
		)

		cmd := map[string]interface{}{
			"command": []interface{}{"show-text", msg, 1000},
		}

		jsonBytes, _ := json.Marshal(cmd)
		conn.Write(jsonBytes)
		conn.Write([]byte("\n"))
	}

	return nil
}
