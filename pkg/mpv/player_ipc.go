package mpv

import (
	"fmt"
)

// IPCPlayer IPC Socket 实现
type IPCPlayer struct {
	ctrl    *Controller
	monitor *IPCMonitor // ✅ 改用 IPCMonitor
}

// NewIPCPlayer 创建 IPC 播放器
func NewIPCPlayer(socketPath string) (*IPCPlayer, error) {
	ctrl, err := NewController(socketPath)
	if err != nil {
		return nil, err
	}

	monitor, err := NewIPCMonitor(socketPath) // ✅ 使用 NewIPCMonitor
	if err != nil {
		ctrl.Close()
		return nil, err
	}

	return &IPCPlayer{
		ctrl:    ctrl,
		monitor: monitor,
	}, nil
}

// Seek 跳转
func (p *IPCPlayer) Seek(seconds float64, mode string) error {
	return p.ctrl.Seek(seconds, mode)
}

// Pause 暂停
func (p *IPCPlayer) Pause() error {
	return p.ctrl.Pause()
}

// Play 播放
func (p *IPCPlayer) Play() error {
	return p.ctrl.Play()
}

// Stop 停止
func (p *IPCPlayer) Stop() error {
	return p.ctrl.Stop()
}

// GetDuration 获取时长
func (p *IPCPlayer) GetDuration() (float64, error) {
	return p.ctrl.GetDuration()
}

// GetTimePos 获取当前时间
func (p *IPCPlayer) GetTimePos() (float64, error) {
	status, ok := p.monitor.GetCurrentStatus()
	if !ok {
		return 0, fmt.Errorf("status not available")
	}
	return status.Timestamp, nil
}

// IsPaused 是否暂停
func (p *IPCPlayer) IsPaused() (bool, error) {
	status, ok := p.monitor.GetCurrentStatus()
	if !ok {
		return false, fmt.Errorf("status not available")
	}
	return status.Paused, nil
}

// ShowText OSD 文本
func (p *IPCPlayer) ShowText(text string, duration int) error {
	return p.ctrl.ShowText(text, duration)
}

// GetMonitor 获取监听器
func (p *IPCPlayer) GetMonitor() *IPCMonitor { // ✅ 返回 IPCMonitor
	return p.monitor
}

// Close 关闭
func (p *IPCPlayer) Close() error {
	p.monitor.Stop()
	return p.ctrl.Close()
}

func (p *IPCPlayer) LoadFile(url string) error {
	// 方式 1: 如果 Controller 有 SendCommand 方法
	return nil // this should not be called in unix

	// 方式 2: 如果视频已在启动时加载，返回 nil
	// return nil
}
