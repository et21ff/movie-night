//go:build windows
// +build windows

package mpv

import (
	"time"

	"movie-night/model"

	"github.com/gen2brain/go-mpv"
)

// LibMPVMonitor libmpv 监听器
type LibMPVMonitor struct {
	mpv      *mpv.Mpv
	statusCh chan model.PlayStatus
	stopCh   chan struct{}
}

// NewLibMPVMonitor 创建监听器
func NewLibMPVMonitor(m *mpv.Mpv) *LibMPVMonitor {
	return &LibMPVMonitor{
		mpv:      m,
		statusCh: make(chan model.PlayStatus, 1),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动监听
func (m *LibMPVMonitor) Start() {
	go m.pollStatus()
}

// pollStatus 轮询状态
func (m *LibMPVMonitor) pollStatus() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			status, _ := m.GetCurrentStatus()

			// 非阻塞发送
			select {
			case m.statusCh <- status:
			default:
				// 队列满，丢弃旧的
				select {
				case <-m.statusCh:
					m.statusCh <- status
				default:
				}
			}
		}
	}
}

// getCurrentStatus 获取当前状态
func (m *LibMPVMonitor) GetCurrentStatus() (model.PlayStatus, bool) {
	var status model.PlayStatus

	// 获取时间
	if val, err := m.mpv.GetProperty("time-pos", mpv.FormatDouble); err == nil && val != nil {
		status.Timestamp = val.(float64)
	}

	// 获取暂停状态
	if val, err := m.mpv.GetProperty("pause", mpv.FormatFlag); err == nil && val != nil {
		status.Paused = val.(bool)
	}

	return status, true
}

// GetStatusChannel 获取状态通道
func (m *LibMPVMonitor) GetStatusChannel() <-chan model.PlayStatus {
	return m.statusCh
}

// Stop 停止监听
func (m *LibMPVMonitor) Stop() {
	close(m.stopCh)
}
