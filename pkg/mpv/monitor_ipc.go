package mpv

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"movie-night/model"
)

// DialSocket 连接到 Unix Socket
func DialSocket(socketPath string) (net.Conn, error) {
	return net.Dial("unix", socketPath)
}

// MPVEvent MPV 事件结构
type MPVEvent struct {
	Event string      `json:"event"`
	Name  string      `json:"name"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

// IPCMonitor IPC Socket 监听器（Unix 系统）
type IPCMonitor struct {
	socketPath string
	conn       net.Conn
	statusCh   chan model.PlayStatus
	stopCh     chan struct{}
}

// NewIPCMonitor 创建 IPC 监听器
func NewIPCMonitor(socketPath string) (*IPCMonitor, error) {
	var conn net.Conn
	var err error

	for i := 0; i < 20; i++ {
		conn, err = DialSocket(socketPath)
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if err != nil {
		return nil, fmt.Errorf("连接 MPV 失败: %w", err)
	}

	m := &IPCMonitor{
		socketPath: socketPath,
		conn:       conn,
		statusCh:   make(chan model.PlayStatus, 1),
		stopCh:     make(chan struct{}),
	}

	commands := []string{
		`{"command": ["observe_property", 1, "time-pos"]}`,
		`{"command": ["observe_property", 2, "pause"]}`,
	}

	for _, cmd := range commands {
		conn.Write([]byte(cmd + "\n"))
	}

	fmt.Println("👂 [Monitor] 开始监听 MPV 播放状态 (IPC)")

	return m, nil
}

// Start 启动监听
func (m *IPCMonitor) Start() {
	go m.listen()
}

// GetStatusChannel 获取状态 channel（只读）
func (m *IPCMonitor) GetStatusChannel() <-chan model.PlayStatus {
	return m.statusCh
}

// listen 监听循环
func (m *IPCMonitor) listen() {
	decoder := json.NewDecoder(m.conn)
	currentStatus := model.PlayStatus{}

	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		var event MPVEvent
		if err := decoder.Decode(&event); err != nil {
			log.Printf("❌ [Monitor] MPV 连接断开: %v", err)
			return
		}

		if m.handleEvent(&event, &currentStatus) {
			select {
			case m.statusCh <- currentStatus:
			default:
				select {
				case <-m.statusCh:
					m.statusCh <- currentStatus
				default:
				}
			}
		}
	}
}

// handleEvent 处理单个事件
func (m *IPCMonitor) handleEvent(event *MPVEvent, currentStatus *model.PlayStatus) bool {
	if event.Event != "property-change" {
		return false
	}

	updated := false

	switch event.Name {
	case "time-pos":
		if seconds, ok := event.Data.(float64); ok {
			currentStatus.Timestamp = seconds
			updated = true
		}
	case "pause":
		if isPaused, ok := event.Data.(bool); ok {
			currentStatus.Paused = isPaused
			updated = true
		}
	}

	return updated
}

// GetCurrentStatus 获取当前状态（非阻塞）
func (m *IPCMonitor) GetCurrentStatus() (model.PlayStatus, bool) {
	select {
	case status := <-m.statusCh:
		select {
		case m.statusCh <- status:
			return status, true
		default:
			return status, true
		}
	default:
		return model.PlayStatus{}, false
	}
}

// Stop 停止监听
func (m *IPCMonitor) Stop() {
	close(m.stopCh)
	if m.conn != nil {
		m.conn.Close()
	}
}
