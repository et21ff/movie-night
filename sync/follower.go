package sync

import (
	"fmt"

	"movie-night/pkg/mpv"
)

// Follower 跟随端（观众）
type Follower struct {
	syncer     *Syncer
	mqttClient *MQTTClient
}

// NewFollowerWithPlayer 创建跟随端
func NewFollowerWithPlayer(player mpv.Player, mqttClient *MQTTClient, maxDuration float64) *Follower {
	return &Follower{
		syncer:     NewSyncer(player, maxDuration),
		mqttClient: mqttClient,
	}
}

// Start 启动跟随端
func (f *Follower) Start() error {
	fmt.Println("📺 跟随端启动")

	// 启动同步器
	f.syncer.Start()

	// 订阅 MQTT
	if err := f.mqttClient.Subscribe(f.syncer.HandleStatus); err != nil {
		return fmt.Errorf("订阅失败: %w", err)
	}

	fmt.Println("✅ 已订阅，等待同步命令")
	return nil
}

// Stop 停止跟随端
func (f *Follower) Stop() {
	f.syncer.Stop()
}
