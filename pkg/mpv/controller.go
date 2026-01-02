package mpv

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Controller struct {
	SocketPath string
	conn       net.Conn
	mu         sync.Mutex
}

func NewController(socketPath string) (*Controller, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MPV: %w", err)
	}

	c := &Controller{
		SocketPath: socketPath,
		conn:       conn,
	}

	// Start a goroutine to drain the connection to prevent blocking
	go func() {
		buffer := make([]byte, 1024)
		for {
			_, err := conn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					// fmt.Println("MPV connection read error:", err)
				}
				return
			}
		}
	}()

	return c, nil
}

// Close closes the connection to MPV
func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// 基础发送逻辑
func (c *Controller) sendCommand(cmdArgs ...interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection is closed")
	}

	payload := map[string]interface{}{
		"command": cmdArgs,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	if _, err := c.conn.Write(append(bytes, '\n')); err != nil {
		return fmt.Errorf("failed to write to MPV: %w", err)
	}
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

func (c *Controller) Pause() error {
	return c.sendCommand("set_property", "pause", true)
}

// Play 开始/继续播放
func (c *Controller) Play() error {
	return c.sendCommand("set_property", "pause", false)
}

func (c *Controller) GetDuration() (float64, error) {
	// 使用临时连接，避免干扰主连接
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return 0, fmt.Errorf("连接 MPV 失败: %w", err)
	}
	defer conn.Close()

	// 设置超时
	conn.SetDeadline(time.Now().Add(2 * time.Second))

	// 发送查询命令
	cmd := `{"command": ["get_property", "duration"]}` + "\n"
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return 0, fmt.Errorf("发送命令失败: %w", err)
	}

	// 读取响应
	decoder := json.NewDecoder(conn)
	var response struct {
		Data  interface{} `json:"data"` // 可能是 float64 或 null
		Error string      `json:"error"`
	}

	if err := decoder.Decode(&response); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查错误
	if response.Error != "" && response.Error != "success" {
		return 0, fmt.Errorf("MPV 错误: %s", response.Error)
	}

	// 检查数据
	if response.Data == nil {
		return 0, fmt.Errorf("视频未加载完成，时长未知")
	}

	// 转换为 float64
	duration, ok := response.Data.(float64)
	if !ok {
		return 0, fmt.Errorf("时长格式错误: %T", response.Data)
	}

	return duration, nil
}
