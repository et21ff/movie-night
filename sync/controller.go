package sync

import (
	"encoding/json"
	"fmt"
	"time"

	"movie-night/model"
	"movie-night/pkg/mpv"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Controller 控制端
type Controller struct {
	mqttClient mqtt.Client // ← 改为原始 client
	topic      string      // ← 添加 topic
	monitor    *mpv.Monitor
	interval   time.Duration
}

// NewController 创建控制端
func NewController(client mqtt.Client, topic string, monitor *mpv.Monitor, interval time.Duration) *Controller {
	return &Controller{
		mqttClient: client,
		topic:      topic,
		monitor:    monitor,
		interval:   interval,
	}
}

// Start 开始广播
func (c *Controller) Start() {
	fmt.Printf("🎮 [Controller] 启动 (每 %v 广播一次)\n", c.interval)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	statusCh := c.monitor.GetStatusChannel()
	var currentStatus model.PlayStatus

	for {
		select {
		case <-ticker.C:
			// ===== 使用原始方式发布 =====
			jsonData, err := json.Marshal(currentStatus)
			if err != nil {
				fmt.Printf("❌ [Controller] 序列化失败: %v\n", err)
				continue
			}

			token := c.mqttClient.Publish(c.topic, 1, true, jsonData)
			token.Wait()

			if token.Error() != nil {
				fmt.Printf("❌ [Controller] 广播失败: %v\n", token.Error())
			} else {
				emoji := "▶️"
				if currentStatus.Paused {
					emoji = "⏸️"
				}
				fmt.Printf("📤 [Controller] 广播: %.2f秒 %s\n", currentStatus.Timestamp, emoji)
			}

		case status := <-statusCh:
			currentStatus = status
		}
	}
}
