package sync

import (
	"fmt"
	"time"

	"movie-night/model"
	"movie-night/pkg/mpv"
)

// Syncer 同步器
type Syncer struct {
	mpvCtrl   mpv.Player
	validator *Validator
	statusCh  chan model.PlayStatus
}

// NewSyncer 创建同步器
func NewSyncer(player mpv.Player, maxDuration float64) *Syncer {
	return &Syncer{
		mpvCtrl:   player,
		validator: NewValidator(maxDuration),
		statusCh:  make(chan model.PlayStatus, 1), // 只保留最新状态
	}
}

// HandleStatus 处理新的播放状态
func (s *Syncer) HandleStatus(status model.PlayStatus) {
	// 1. 验证状态
	if err := s.validator.Validate(status); err != nil {
		fmt.Printf("⚠️  状态无效: %v\n", err)
		return
	}

	// 2. 显示接收信息
	pausedStr := "▶️"
	if status.Paused {
		pausedStr = "⏸️"
	}
	fmt.Printf("📥 收到: %.2f秒 %s\n", status.Timestamp, pausedStr)

	// 3. 发送到处理队列（非阻塞，只保留最新）
	select {
	case s.statusCh <- status:
		// 成功发送
	default:
		// 队列满，丢弃旧的，保留新的
		select {
		case <-s.statusCh:
			s.statusCh <- status
			fmt.Println("⚠️  更新为最新状态")
		default:
		}
	}
}

// Start 启动同步处理
func (s *Syncer) Start() {
	go s.processLoop()
}

// processLoop 处理循环
func (s *Syncer) processLoop() {
	for status := range s.statusCh {
		s.syncToMPV(status)
	}
}

// syncToMPV 同步到 MPV
func (s *Syncer) syncToMPV(status model.PlayStatus) {
	fmt.Printf("🎬 同步: %.2f秒, 暂停=%v\n", status.Timestamp, status.Paused)

	// 1. 跳转到指定位置
	if err := s.mpvCtrl.Seek(status.Timestamp, "absolute"); err != nil {
		fmt.Printf("❌ 跳转失败: %v\n", err)
		return
	}

	// 2. 短暂延迟，让跳转完成
	time.Sleep(50 * time.Millisecond)

	// 3. 设置暂停状态
	if status.Paused {
		// MPV 已经暂停，不需要操作
		// 或者发送暂停命令
		s.mpvCtrl.Pause()
	} else {
		// 确保播放
		s.mpvCtrl.Play()
		// s.mpvCtrl.sendCommand("set_property", "pause", false)
	}
}

// Stop 停止同步
func (s *Syncer) Stop() {
	close(s.statusCh)
}
