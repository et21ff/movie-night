// pkg/share/code.go
package share

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// RoomInfo 房间信息
type RoomInfo struct {
	MagnetLink string `json:"m"`
	RoomID     string `json:"r"`
}

// Encode 编码房间信息为分享码
func Encode(magnetLink, roomID string) (string, error) {
	info := RoomInfo{
		MagnetLink: magnetLink,
		RoomID:     roomID,
	}

	// JSON 序列化
	jsonData, err := json.Marshal(info)
	if err != nil {
		return "", err
	}

	// Gzip 压缩
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(jsonData); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}

	// Base64 编码
	code := base64.URLEncoding.EncodeToString(buf.Bytes())

	return code, nil
}

// Decode 解码分享码
func Decode(code string) (*RoomInfo, error) {
	// Base64 解码
	compressed, err := base64.URLEncoding.DecodeString(code)
	if err != nil {
		// 尝试标准 Base64
		compressed, err = base64.StdEncoding.DecodeString(code)
		if err != nil {
			return nil, fmt.Errorf("无效的分享码")
		}
	}

	// Gzip 解压
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("解压失败")
	}
	defer gz.Close()

	jsonData, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("读取失败")
	}

	// JSON 反序列化
	var info RoomInfo
	if err := json.Unmarshal(jsonData, &info); err != nil {
		return nil, fmt.Errorf("解析失败")
	}

	return &info, nil
}
