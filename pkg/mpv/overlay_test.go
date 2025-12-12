package mpv

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Mock MPV Server to capture commands
func startMockMpvServer(t *testing.T, socketPath string, cmdChan chan string) {
	os.Remove(socketPath)
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to listen on socket: %v", err)
	}
	go func() {
		defer l.Close()
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go handleConnection(conn, cmdChan)
		}
	}()
}

func handleConnection(conn net.Conn, cmdChan chan string) {
	defer conn.Close()
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	cmdChan <- string(buf[:n])
}

func TestDrawSyncOverlay(t *testing.T) {
	// Setup Mock Server
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "mpv-test.sock")
	cmdChan := make(chan string, 10)
	startMockMpvServer(t, socketPath, cmdChan)

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	controller := NewController(socketPath)

	// Test Data
	states := map[string]PeerSyncState{
		"Alice": {Name: "Alice", IsReady: true},
		"Bob":   {Name: "Bob", IsReady: false, Buffering: 50, StatusText: "Buffering"},
	}

	// Execute
	err := controller.DrawSyncOverlay(states)
	if err != nil {
		t.Fatalf("DrawSyncOverlay failed: %v", err)
	}

	// Verify
	select {
	case cmdJSON := <-cmdChan:
		var payload struct {
			Command []interface{} `json:"command"`
		}
		if err := json.Unmarshal([]byte(cmdJSON), &payload); err != nil {
			t.Fatalf("Failed to parse command JSON: %v", err)
		}

		// Check Command Structure
		// ["osd-overlay", 1, "ass-events", <content>]
		if len(payload.Command) != 4 {
			t.Fatalf("Unexpected command length: %d", len(payload.Command))
		}
		if payload.Command[0] != "osd-overlay" {
			t.Errorf("Expected command 'osd-overlay', got %v", payload.Command[0])
		}
		if fmt.Sprintf("%v", payload.Command[1]) != "1" {
			t.Errorf("Expected overlay id 1, got %v", payload.Command[1])
		}

		assContent := payload.Command[3].(string)
		t.Logf("Generated ASS: %s", assContent)

		// Check ASS Content
		if !strings.Contains(assContent, "Sync Status") {
			t.Error("ASS content missing title")
		}
		if !strings.Contains(assContent, "Alice") {
			t.Error("ASS content missing Alice")
		}
		if !strings.Contains(assContent, "Bob") {
			t.Error("ASS content missing Bob")
		}
		// Check Colors
		if !strings.Contains(assContent, `\c&H00FF00&`) { // Green for Ready
			t.Error("ASS content missing Green color for Ready")
		}
		if !strings.Contains(assContent, `\c&H00FFFF&`) { // Yellow for Not Ready
			t.Error("ASS content missing Yellow color for Not Ready")
		}

	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for command")
	}
}

func TestClearSyncOverlay(t *testing.T) {
	// Setup Mock Server
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "mpv-test-clear.sock")
	cmdChan := make(chan string, 10)
	startMockMpvServer(t, socketPath, cmdChan)
	time.Sleep(100 * time.Millisecond)

	controller := NewController(socketPath)

	err := controller.ClearSyncOverlay()
	if err != nil {
		t.Fatalf("ClearSyncOverlay failed: %v", err)
	}

	select {
	case cmdJSON := <-cmdChan:
		var payload struct {
			Command []interface{} `json:"command"`
		}
		json.Unmarshal([]byte(cmdJSON), &payload)
		assContent := payload.Command[3].(string)
		if assContent != "" {
			t.Errorf("Expected empty ASS content for clear, got '%s'", assContent)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for command")
	}
}
