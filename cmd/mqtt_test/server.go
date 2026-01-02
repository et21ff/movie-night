package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// 播放状态
type PlayStatus struct {
	Timestamp float64 `json:"timestamp"` // 时间轴
	Paused    bool    `json:"paused"`    // 是否暂停
}

func main() {
	fmt.Println("🎬 视频播放控制器启动\n")

	// 随机数种子
	rand.Seed(time.Now().UnixNano())

	// 连接 MQTT
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://broker-cn.emqx.io:1883")
	opts.SetClientID("video-controller")

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	fmt.Println("✅ 已连接到 MQTT Broker")
	fmt.Println("📡 发送频道: video/control\n")

	// 每 10 秒发送一次
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		// 生成随机状态
		status := PlayStatus{
			Timestamp: rand.Float64() * 100, // 0-100 随机
			Paused:    rand.Intn(2) == 0,    // 随机 true/false
		}

		// 转 JSON
		jsonData, _ := json.Marshal(status)

		// 发布
		token := client.Publish("video/control", 1, true, jsonData)
		token.Wait()

		// 打印
		pausedStr := "播放中 ▶️"
		if status.Paused {
			pausedStr = "暂停 ⏸️"
		}

		fmt.Printf("📤 [%s] 时间轴: %.2f 秒, 状态: %s\n",
			time.Now().Format("15:04:05"),
			status.Timestamp,
			pausedStr)
	}
}
