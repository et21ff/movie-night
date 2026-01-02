package config

// Config 应用配置
type Config struct {
	// MQTT 配置
	MQTTBroker   string
	MQTTClientID string
	MQTTTopic    string

	// 其他配置
	VideoDuration float64
}

// Default 返回默认配置
func Default() *Config {
	return &Config{
		MQTTBroker:    "tcp://broker-cn.emqx.io:1883",
		MQTTClientID:  "video-client",
		MQTTTopic:     "video/control",
		VideoDuration: 0, // 0 表示不限制
	}
}
