package sync

import (
	"fmt"
	"math"

	"movie-night/model"
)

// Validator 状态验证器
type Validator struct {
	MaxDuration float64 // 视频最大时长
}

// NewValidator 创建验证器
func NewValidator(maxDuration float64) *Validator {
	return &Validator{
		MaxDuration: maxDuration,
	}
}

// Validate 验证播放状态
func (v *Validator) Validate(status model.PlayStatus) error {
	// 1. 检查时间轴范围
	if status.Timestamp < 0 {
		return fmt.Errorf("时间轴不能为负: %.2f", status.Timestamp)
	}

	if v.MaxDuration > 0 && status.Timestamp > v.MaxDuration {
		return fmt.Errorf("时间轴超出范围: %.2f > %.2f",
			status.Timestamp, v.MaxDuration)
	}

	// 2. 检查特殊浮点数
	if math.IsNaN(status.Timestamp) {
		return fmt.Errorf("时间轴是 NaN")
	}

	if math.IsInf(status.Timestamp, 0) {
		return fmt.Errorf("时间轴是 Infinity")
	}

	return nil
}
