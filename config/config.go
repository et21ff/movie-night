package config

import "movie-night/pkg/mpv"

// Config 应用配置
type Config struct {
	// P2P 配置
	MagnetLink string
	DataDir    string
	MaxConns   int

	// HTTP 配置
	StreamPort int

	// MPV 配置
	MPVSocketPath string
	VideoDuration float64

	// MQTT 配置
	MQTTBroker   string
	MQTTClientID string
	MQTTTopic    string
}

// Default 返回默认配置
func Default() *Config {
	return &Config{
		// P2P 默认值
		MagnetLink: "magnet:?xt=urn:btih:JEJJEE6LGDVRMHT7XVJGJ74BKVW6WL2M&dn=&tr=http%3A%2F%2F104.143.10.186%3A8000%2Fannounce&tr=udp%3A%2F%2F104.143.10.186%3A8000%2Fannounce",
		DataDir:    "./downloads",
		MaxConns:   50,

		// HTTP
		StreamPort: 8888,

		// MPV
		MPVSocketPath: mpv.DefaultSocketPath(),
		VideoDuration: 0, // 0 表示不限制

		// MQTT
		MQTTBroker:   "tcp://broker-cn.emqx.io:1883",
		MQTTClientID: "video-client",
		MQTTTopic:    "video/control",
	}
}
