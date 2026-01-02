package mpv

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultSocketPath 获取默认 Socket 路径（跨平台）
func DefaultSocketPath() string {
	if runtime.GOOS == "windows" {
		// Windows 使用 TCP Socket
		return "127.0.0.1:28888"
	}
	// Unix 使用文件 Socket
	return filepath.Join(os.TempDir(), "mpv-socket")
}

// DialSocket 连接 MPV Socket（跨平台）
func DialSocket(socketPath string) (net.Conn, error) {
	if strings.Contains(socketPath, ":") {
		// TCP 模式（Windows 或手动指定）
		return net.Dial("tcp", socketPath)
	}
	// Unix Socket 模式（Linux/macOS）
	return net.Dial("unix", socketPath)
}

// CleanupSocket 清理旧的 Socket 文件（仅 Unix）
func CleanupSocket(socketPath string) {
	if runtime.GOOS == "windows" {
		// Windows 的 TCP Socket 不需要清理
		return
	}

	// Unix: 删除旧的 Socket 文件
	if _, err := os.Stat(socketPath); err == nil {
		os.Remove(socketPath)
	}
}
