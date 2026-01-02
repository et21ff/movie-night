package mpv

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"movie-night/model"
)

// MPVEvent MPV 事件
type MPVEvent struct {
	Event string      `json:"event"`
	Name  string      `json:"name"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

// Monitor MPV 状态监听器
type Monitor struct {
	socketPath string
	conn       net.Conn
	statusCh   chan model.PlayStatus // 状态 channel
	stopCh     chan struct{}
}

// NewMonitor 创建监听器
func NewMonitor(socketPath string) (*Monitor, error) {
	// 等待 Socket 就绪
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

	m := &Monitor{
		socketPath: socketPath,
		conn:       conn,
		statusCh:   make(chan model.PlayStatus, 1), // 只保留最新状态
		stopCh:     make(chan struct{}),
	}

	// 发送监听命令
	commands := []string{
		`{"command": ["observe_property", 1, "time-pos"]}`,
		`{"command": ["observe_property", 2, "pause"]}`,
	}

	for _, cmd := range commands {
		conn.Write([]byte(cmd + "\n"))
	}

	fmt.Println("👂 [Monitor] 开始监听 MPV 播放状态")

	return m, nil
}

// Start 启动监听
func (m *Monitor) Start() {
	go m.listen()
}

// GetStatusChannel 获取状态 channel（只读）
func (m *Monitor) GetStatusChannel() <-chan model.PlayStatus {
	return m.statusCh
}

// listen 监听循环
func (m *Monitor) listen() {
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

		// 处理事件
		if event.Event == "property-change" {
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

			// 有更新时发送到 channel（非阻塞）
			if updated {
				select {
				case m.statusCh <- currentStatus:
					// 成功发送
				default:
					// channel 满，丢弃旧的，发送新的
					select {
					case <-m.statusCh:
						m.statusCh <- currentStatus
					default:
					}
				}
			}
		}
	}
}

// GetCurrentStatus 获取当前状态（同步）
func (m *Monitor) GetCurrentStatus() model.PlayStatus {
	select {
	case status := <-m.statusCh:
		// 读取后立即放回
		m.statusCh <- status
		return status
	default:
		return model.PlayStatus{}
	}
}

// Stop 停止监听
func (m *Monitor) Stop() {
	close(m.stopCh)
	if m.conn != nil {
		m.conn.Close()
	}
}
