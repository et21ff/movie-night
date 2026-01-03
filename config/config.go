package config

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
		MagnetLink: "magnet:?xt=urn:btih:6926967ed2d4a6112a06594e6b05eedc215424d6&dn=%5BToonsHub%5D%20Millennium%20Actress%20%282002%29%20REPACK%202160p%20HMAX%20WEB-DL%20DDP5.1%20H.265%2010bit%20%28Sennen%20Joyuu%2C%20Dual-Audio%2C%20Multi-Subs%29&tr=http%3A%2F%2Fnyaa.tracker.wf%3A7777%2Fannounce&tr=udp%3A%2F%2Fopen.stealth.si%3A80%2Fannounce&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce&tr=udp%3A%2F%2Fexodus.desync.com%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.torrent.eu.org%3A451%2Fannounce",
		DataDir:    "./downloads",
		MaxConns:   50,

		// HTTP
		StreamPort: 8888,

		// MPV
		MPVSocketPath: "/tmp/mpv-socket",
		VideoDuration: 0, // 0 表示不限制

		// MQTT
		MQTTBroker:   "tcp://broker-cn.emqx.io:1883",
		MQTTClientID: "video-client",
		MQTTTopic:    "video/control",
	}
}
