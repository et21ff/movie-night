package sync

import (
	"encoding/json"
	"fmt"
	"time"

	"movie-night/model"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Controller 控制端（房主）
type Controller struct {
	mqttClient mqtt.Client // ← 原始 MQTT client
	topic      string      // ← MQTT 主题
	statusCh   <-chan model.PlayStatus
	interval   time.Duration
}

// NewController 创建控制端
func NewController(client mqtt.Client, topic string, statusCh <-chan model.PlayStatus, interval time.Duration) *Controller {
	return &Controller{
		mqttClient: client,
		topic:      topic,
		statusCh:   statusCh,
		interval:   interval,
	}
}

// Start 开始广播
func (c *Controller) Start() {
	fmt.Printf("🎮 [Controller] 启动 (每 %v 广播一次)\n", c.interval)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	var currentStatus model.PlayStatus

	for {
		select {
		case <-ticker.C:
			// ===== 手动序列化并发布 =====
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

		case status := <-c.statusCh:
			// 实时更新本地状态
			currentStatus = status
		}
	}
}

// Stop 停止控制端
func (c *Controller) Stop() {
	// 清理逻辑（如果需要）
}
