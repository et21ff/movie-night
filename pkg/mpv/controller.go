package mpv

import (
	"encoding/json"
	"fmt"
	"net"
)

type Controller struct {
	SocketPath string
}

func NewController(socketPath string) *Controller {
	return &Controller{SocketPath: socketPath}
}

// 基础发送逻辑
func (c *Controller) sendCommand(cmdArgs ...interface{}) error {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return fmt.Errorf("连接 MPV 失败: %w", err)
	}
	defer conn.Close()

	payload := map[string]interface{}{
		"command": cmdArgs,
	}
	bytes, _ := json.Marshal(payload)
	conn.Write(bytes)
	conn.Write([]byte("\n"))
	return nil
}

// === 业务方法 ===

func (c *Controller) CyclePause() error {
	return c.sendCommand("cycle", "pause")
}

func (c *Controller) Seek(seconds float64, mode string) error {
	if mode == "" {
		mode = "absolute"
	}
	return c.sendCommand("seek", seconds, mode)
}

func (c *Controller) ShowText(text string, duration int) error {
	return c.sendCommand("show-text", text, duration)
}
