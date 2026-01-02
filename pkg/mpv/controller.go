package mpv

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
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
