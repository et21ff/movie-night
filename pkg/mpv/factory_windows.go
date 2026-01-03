// pkg/mpv/factory_windows.go
//go:build windows

package mpv

import "fmt"

// NewPlayer 创建播放器（Windows: libmpv）
func NewPlayer() (Player, error) {
	fmt.Println("📺 [MPV] 使用 libmpv 模式")
	return NewLibMPVPlayer()
}

// NewMonitorFromPlayer 从播放器获取监控器
func NewMonitorFromPlayer(p Player) MonitorInterface {
	if lp, ok := p.(*LibMPVPlayer); ok {
		return lp.GetMonitor()
	}
	return nil
}
