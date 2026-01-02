package main

import (
	"encoding/json"
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// 播放状态
type PlayStatus struct {
	Timestamp float64 `json:"timestamp"`
	Paused    bool    `json:"paused"`
}

func main() {
	fmt.Println("📺 视频客户端启动\n")

	// 连接 MQTT
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://broker-cn.emqx.io:1883")
	opts.SetClientID("video-client-1") // 多个客户端改这里
	opts.SetCleanSession(false)        // 保存离线消息

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	fmt.Println("✅ 已连接到 MQTT Broker")
	fmt.Println("📡 订阅频道: video/control\n")

	// 订阅
	token := client.Subscribe("video/control", 1, func(c mqtt.Client, m mqtt.Message) {
		var status PlayStatus
		if err := json.Unmarshal(m.Payload(), &status); err != nil {
			fmt.Println("❌ 解析失败:", err)
			return
		}

		// 显示接收到的状态
		pausedStr := "播放中 ▶️"
		if status.Paused {
			pausedStr = "暂停 ⏸️"
		}

		fmt.Printf("📥 同步: 时间轴 %.2f 秒, 状态: %s\n",
			status.Timestamp,
			pausedStr)
	})

	token.Wait()
	if token.Error() != nil {
		panic(token.Error())
	}

	fmt.Println("⏳ 等待控制器消息...\n")

	// 保持运行
	select {}
}
