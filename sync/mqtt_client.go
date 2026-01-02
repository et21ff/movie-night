package sync

import (
	"encoding/json"
	"fmt"
	"time"

	"movie-night/model"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient MQTT 客户端封装
type MQTTClient struct {
	client mqtt.Client
	topic  string
}

// MQTTConfig MQTT 配置
type MQTTConfig struct {
	Broker   string // MQTT Broker 地址
	ClientID string // 客户端 ID
	Topic    string // 订阅主题
}

// NewMQTTClient 创建 MQTT 客户端
func NewMQTTClient(config MQTTConfig) (*MQTTClient, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.Broker)
	opts.SetClientID(config.ClientID)
	opts.SetCleanSession(false)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetAutoReconnect(true)

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Println("✅ MQTT 已连接")
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		fmt.Printf("❌ MQTT 连接丢失: %v\n", err)
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()

	if !token.WaitTimeout(5 * time.Second) {
		return nil, fmt.Errorf("MQTT 连接超时")
	}

	if token.Error() != nil {
		return nil, fmt.Errorf("MQTT 连接失败: %w", token.Error())
	}

	return &MQTTClient{
		client: client,
		topic:  config.Topic,
	}, nil
}

// Subscribe 订阅主题
func (m *MQTTClient) Subscribe(handler func(model.PlayStatus)) error {
	token := m.client.Subscribe(m.topic, 1, func(c mqtt.Client, msg mqtt.Message) {
		var status model.PlayStatus
		if err := json.Unmarshal(msg.Payload(), &status); err != nil {
			fmt.Printf("❌ JSON 解析失败: %v\n", err)
			return
		}

		// 调用处理函数
		handler(status)
	})

	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("订阅失败: %w", token.Error())
	}

	fmt.Printf("📡 已订阅: %s\n", m.topic)
	return nil
}

// Close 关闭连接
func (m *MQTTClient) Close() {
	if m.client != nil && m.client.IsConnected() {
		m.client.Disconnect(250)
	}
}

func (m *MQTTClient) GetClient() mqtt.Client {
	return m.client
}

// GetTopic 获取主题
func (m *MQTTClient) GetTopic() string {
	return m.topic
}
