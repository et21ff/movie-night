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
	server     *http.Server
	listener   net.Listener
	mu         sync.Mutex
	running    bool
}

func NewStreamServer(port int, file *torrent.File) *StreamServer {
	// ✅ 创建时就标记文件为需要下载
	if file != nil {
		file.Download()
		fmt.Printf("📥 [Stream] 标记下载: %s\n", file.DisplayPath())
	}

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

	// ✅ 启动时再次确保文件被标记下载
	if s.targetFile != nil {
		s.targetFile.Download()

		// ✅ 预热：提前开始下载前面的数据
		s.preheat()
	}

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)

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
	}

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	fmt.Printf("📡 [HTTP] 流服务: http://127.0.0.1:%d/stream\n", s.port)

	err = s.server.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// ✅ 预热：提前开始下载
func (s *StreamServer) preheat() {
	if s.targetFile == nil {
		return
	}

	// 创建一个临时 reader 来触发下载
	reader := s.targetFile.NewReader()
	reader.SetResponsive()
	reader.SetReadahead(20 << 20) // 20MB 预读

	// 读取前 1KB 触发下载
	buf := make([]byte, 1024)
	go func() {
		n, err := reader.Read(buf)
		if err != nil {
			fmt.Printf("⚠️ [Stream] 预热读取失败: %v\n", err)
		} else {
			fmt.Printf("✅ [Stream] 预热成功，读取 %d 字节\n", n)
		}
		// 注意：不要关闭这个 reader，让它继续预读
	}()

	fmt.Println("🔥 [Stream] 开始预热下载...")
}

func (s *StreamServer) handleStream(w http.ResponseWriter, r *http.Request) {
	if s.targetFile == nil {
		http.Error(w, "No file", http.StatusNotFound)
		return
	}

	reader := s.targetFile.NewReader()
	reader.SetResponsive()
	reader.SetReadahead(50 << 20) // ✅ 增加到 50MB 预读

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

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}

	fmt.Println("✅ [HTTP] 流服务已关闭")
	return nil
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
