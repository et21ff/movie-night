// pkg/mpv/interface_check_unix.go
//go:build !windows

package mpv

var (
	_ Player           = (*IPCPlayer)(nil)
	_ MonitorInterface = (*IPCMonitor)(nil)
)
