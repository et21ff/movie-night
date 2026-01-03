//go:build windows
// +build windows

package mpv

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/gen2brain/go-mpv"
)

// LibMPVPlayer libmpv 实现
type LibMPVPlayer struct {
	mpv     *mpv.Mpv
	monitor *LibMPVMonitor
}

// NewLibMPVPlayer 创建 libmpv 播放器
func NewLibMPVPlayer() (*LibMPVPlayer, error) {
	// 设置 DPI 感知
	setDPIAware()

	m := mpv.New()

	// 配置 MPV
	m.SetOptionString("osc", "yes")
	m.SetOptionString("script-opts", "osc-scalefull=1.5,osc-scalewindowed=1.5")
	m.SetOptionString("profile", "gpu-hq")
	m.SetOptionString("vo", "gpu")
	m.SetOptionString("input-default-bindings", "yes")
	// ✅ 启用详细日志
	m.SetOptionString("terminal", "yes")
	m.SetOptionString("msg-level", "all=v") // v=verbose, debug, trace

	// ✅ 输出日志到文件
	m.SetOptionString("log-file", "mpv-debug.log")

	for _, env := range []string{
		"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY",
		"http_proxy", "https_proxy", "all_proxy",
	} {
		os.Unsetenv(env)
	}
	os.Setenv("NO_PROXY", "localhost,127.0.0.1,::1")
	// Windows 特定配置
	if runtime.GOOS == "windows" {
		m.SetOptionString("gpu-context", "d3d11")
		m.SetOptionString("hidpi-window-scale", "yes")
	}

	// 初始化
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("初始化 MPV 失败: %w", err)
	}

	player := &LibMPVPlayer{
		mpv: m,
	}

	// 创建监听器
	player.monitor = NewLibMPVMonitor(m)

	return player, nil
}

// Seek 跳转
func (p *LibMPVPlayer) Seek(seconds float64, mode string) error {
	return p.mpv.SetProperty("time-pos", mpv.FormatDouble, seconds)
}

// Pause 暂停
func (p *LibMPVPlayer) Pause() error {
	return p.mpv.SetProperty("pause", mpv.FormatFlag, true)
}

// Play 播放
func (p *LibMPVPlayer) Play() error {
	return p.mpv.SetProperty("pause", mpv.FormatFlag, false)
}

// Stop 停止
func (p *LibMPVPlayer) Stop() error {
	return p.mpv.Command([]string{"stop"})
}

// GetDuration 获取时长
func (p *LibMPVPlayer) GetDuration() (float64, error) {
	val, err := p.mpv.GetProperty("duration", mpv.FormatDouble)
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, fmt.Errorf("duration not available")
	}
	return val.(float64), nil
}

// GetTimePos 获取当前时间
func (p *LibMPVPlayer) GetTimePos() (float64, error) {
	val, err := p.mpv.GetProperty("time-pos", mpv.FormatDouble)
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, nil
	}
	return val.(float64), nil
}

// IsPaused 是否暂停
func (p *LibMPVPlayer) IsPaused() (bool, error) {
	val, err := p.mpv.GetProperty("pause", mpv.FormatFlag)
	if err != nil {
		return false, err
	}
	if val == nil {
		return false, nil
	}
	return val.(bool), nil
}

// ShowText 显示 OSD 文本
func (p *LibMPVPlayer) ShowText(text string, duration int) error {
	return p.mpv.Command([]string{"show-text", text, fmt.Sprintf("%d", duration)})
}

func (p *LibMPVPlayer) LoadFile(url string) error {
	if p.mpv == nil {
		return fmt.Errorf("mpv 未初始化")
	}

	fmt.Printf("🔧 [DEBUG] 准备加载: %s\n", url)

	// ✅ 使用 CommandString - 直接发送命令字符串
	err := p.mpv.CommandString(fmt.Sprintf("loadfile %s", url))

	if err != nil {
		fmt.Printf("❌ [DEBUG] CommandString 失败: %v\n", err)
		return fmt.Errorf("加载视频失败: %w", err)
	}

	fmt.Println("✅ [DEBUG] 加载命令已发送")
	time.Sleep(time.Second)
	return nil
}

// GetMonitor 获取监听器
func (p *LibMPVPlayer) GetMonitor() *LibMPVMonitor {
	return p.monitor
}

// Close 关闭
func (p *LibMPVPlayer) Close() error {
	if p.monitor != nil {
		p.monitor.Stop()
	}
	p.mpv.TerminateDestroy()
	return nil
}

// WaitForShutdown 等待播放器关闭
func (p *LibMPVPlayer) WaitForShutdown() {
	for {
		event := p.mpv.WaitEvent(10)
		if event.EventID == mpv.EventShutdown {
			break
		}
	}
}

// setDPIAware 设置 DPI 感知
func setDPIAware() {
	if runtime.GOOS != "windows" {
		return
	}

	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("SetProcessDPIAware")
	ret, _, _ := proc.Call()

	if ret != 0 {
		fmt.Println("✅ DPI 感知已开启")
	}
}
