package main

import (
    "fmt"

    mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
    fmt.Println("🎬 Client 启动")

    // 连接 MQTT Broker
    opts := mqtt.NewClientOptions()
    opts.AddBroker("tcp://broker-cn.emqx.io:1883")
    opts.SetClientID("client-1")

    client := mqtt.NewClient(opts)
    if token := client.Connect(); token.Wait() && token.Error() != nil {
        panic(token.Error())
    }

    fmt.Println("✅ 已连接到 MQTT Broker")
    fmt.Println("📡 订阅频道: video/sync\n")

    // 订阅固定频道
    topic := "video/sync"

    token := client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
        fmt.Printf("📥 收到: %s\n", string(msg.Payload()))
    })
    token.Wait()

    if token.Error() != nil {
        panic(token.Error())
    }

    fmt.Println("⏳ 等待消息...\n")

    // 保持运行
    select {}
}