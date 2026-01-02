package main

import (
    "fmt"
    "time"

    mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
    fmt.Println("🚀 Server 启动")

    // 连接 MQTT Broker
    opts := mqtt.NewClientOptions()
    opts.AddBroker("tcp://broker-cn.emqx.io:1883")
    opts.SetClientID("server-1")

    client := mqtt.NewClient(opts)
    if token := client.Connect(); token.Wait() && token.Error() != nil {
        panic(token.Error())
    }

    fmt.Println("✅ 已连接到 MQTT Broker")
    fmt.Println("📡 开始发送消息到频道: video/sync\n")

    // 持续发送时间轴
    currentTime := 0.0

    for {
        // 模拟视频播放，每秒增加 1 秒
        currentTime += 1.0

        message := fmt.Sprintf("当前时间: %.1f 秒", currentTime)

        // 发布到固定频道
        token := client.Publish("video/sync", 0, false, message)
        token.Wait()

        fmt.Printf("📤 发送: %s\n", message)

        time.Sleep(1 * time.Second)
    }
}