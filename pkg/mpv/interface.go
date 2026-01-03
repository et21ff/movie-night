package mpv

import (
    "movie-night/model"
)

// Player MPV 播放器接口
type Player interface {
    // 播放控制
    Seek(seconds float64, mode string) error
    Pause() error
    Play() error
    Stop() error
    
    // 属性获取
    GetDuration() (float64, error)
    GetTimePos() (float64, error)
    IsPaused() (bool, error)
    
    // OSD
    ShowText(text string, duration int) error
    
    // 生命周期
    Close() error
}

type MonitorInterface interface {
    Start()
    GetStatusChannel() <-chan model.PlayStatus
    GetCurrentStatus() (model.PlayStatus, bool)
    Stop()
}

// 确保实现了接口
var (
    _ MonitorInterface = (*IPCMonitor)(nil)
    _ MonitorInterface = (*LibMPVMonitor)(nil)
)

// Launcher MPV 启动器接口
type Launcher interface {
    Launch(url string) error
    IsRunning() bool
}