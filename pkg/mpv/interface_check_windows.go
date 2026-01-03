// pkg/mpv/interface_check_windows.go
//go:build windows

package mpv

var (
	_ Player           = (*LibMPVPlayer)(nil)
	_ MonitorInterface = (*LibMPVMonitor)(nil)
)
