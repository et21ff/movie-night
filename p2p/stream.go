package p2p

import (
	"fmt"
	"net/http"
	"time"

	"github.com/anacrolix/torrent"
)

// StreamServer HTTP 流服务器
type StreamServer struct {
	port       int
	targetFile *torrent.File
}

// NewStreamServer 创建流服务器
func NewStreamServer(port int, file *torrent.File) *StreamServer {
	return &StreamServer{
		port:       port,
		targetFile: file,
	}
}

// Start 启动服务器（阻塞）
func (s *StreamServer) Start() error {
	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		reader := s.targetFile.NewReader()
		reader.SetResponsive()
		defer reader.Close()

		http.ServeContent(w, r, s.targetFile.DisplayPath(), time.Now(), reader)
	})

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("📡 [HTTP] 流服务: http://localhost:%d/stream\n", s.port)

	return http.ListenAndServe(addr, nil)
}

// GetURL 获取流地址
func (s *StreamServer) GetURL() string {
	return fmt.Sprintf("http://localhost:%d/stream", s.port)
}
