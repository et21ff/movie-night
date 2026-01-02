package model

// PlayStatus 播放状态
type PlayStatus struct {
	Timestamp float64 `json:"timestamp"` // 当前播放位置（秒）
	Paused    bool    `json:"paused"`    // 是否暂停
}

// IsZero 检查是否为零值
func (s PlayStatus) IsZero() bool {
	return s.Timestamp == 0 && !s.Paused
}
