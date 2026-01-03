package p2p

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

type StreamServer struct {
	port       int
	targetFile *torrent.File
	server     *http.Server // ✅ 保存引用
	listener   net.Listener // ✅ 保存引用
	mu         sync.Mutex
	running    bool
}

func NewStreamServer(port int, file *torrent.File) *StreamServer {
	return &StreamServer{
		port:       port,
		targetFile: file,
	}
}

func (s *StreamServer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("服务器已在运行")
	}
	s.mu.Unlock()

	// ✅ 绑定到 127.0.0.1，不是 0.0.0.0
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)

	// ✅ 先监听，检测端口占用
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("端口 %d 被占用: %w", s.port, err)
	}
	s.listener = listener

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", s.handleStream)

	s.server = &http.Server{
		Handler:     mux,
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 60 * time.Second,
		// WriteTimeout 不设置，流媒体需要持续写入
	}

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	fmt.Printf("📡 [HTTP] 流服务: http://127.0.0.1:%d/stream\n", s.port)

	// ✅ 使用已创建的 listener
	err = s.server.Serve(listener)
	if err == http.ErrServerClosed {
		return nil // 正常关闭
	}
	return err
}

func (s *StreamServer) handleStream(w http.ResponseWriter, r *http.Request) {
	if s.targetFile == nil {
		http.Error(w, "No file", http.StatusNotFound)
		return
	}

	reader := s.targetFile.NewReader()
	reader.SetResponsive()
	reader.SetReadahead(10 << 20) // 10MB 预读

	// 客户端断开时关闭 reader
	go func() {
		<-r.Context().Done()
		reader.Close()
	}()
	defer reader.Close()

	name := s.targetFile.DisplayPath()
	if ct := contentTypeByName(name); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Accept-Ranges", "bytes")

	// ✅ 使用 time.Time{} 避免缓存问题
	http.ServeContent(w, r, name, time.Time{}, reader)
}

// ✅ 优雅关闭
func (s *StreamServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}
	s.running = false

	fmt.Println("🔧 [HTTP] 正在关闭流服务...")

	var err error
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err = s.server.Shutdown(ctx)
	}

	fmt.Println("✅ [HTTP] 流服务已关闭")
	return err
}

func (s *StreamServer) GetURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/stream", s.port)
}

func contentTypeByName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mkv":
		return "video/x-matroska"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	default:
		return "application/octet-stream"
	}
}
