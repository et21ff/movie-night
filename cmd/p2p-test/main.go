package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"movie-night/p2p"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type ChatMessage struct {
	Content string `json:"content"`
	Time    int64  `json:"time"`
}

func main() {
	targetAddr := flag.String("join", "", "要连接的目标节点地址")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("🚀 [Test] 正在启动 P2P 节点...")
	node, err := p2p.NewNode(ctx)
	if err != nil {
		log.Fatalf("创建节点失败: %v", err)
	}

	// ---------------------------------------------------------
	// 【关键修改】打印出系统分配的随机端口
	// ---------------------------------------------------------
	fmt.Println("\n📋 本机监听地址 (包含随机端口):")
	fmt.Println("---------------------------------------------------------")
	for _, addr := range node.Host.Addrs() {
		// 忽略 IPv6 本地回环，只显示更易读的 IPv4
		if !strings.Contains(addr.String(), "::1") {
			// 这里打印出来的端口号就是程序正在使用的端口
			fmt.Printf("%s/p2p/%s\n", addr, node.Host.ID())
		}
	}
	fmt.Println("---------------------------------------------------------\n")

	// 尝试直连逻辑
	if *targetAddr != "" {
		fmt.Printf("🔗 正在尝试直连: %s\n", *targetAddr)
		maddr, err := multiaddr.NewMultiaddr(*targetAddr)
		if err != nil {
			log.Printf("❌ 地址格式错误: %v", err)
		} else {
			info, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				log.Printf("❌ 解析 PeerInfo 失败: %v", err)
			} else {
				if err := node.Host.Connect(ctx, *info); err != nil {
					log.Printf("❌ 连接失败: %v", err)
				} else {
					fmt.Println("✅ 直连成功！")
				}
			}
		}
	}

	// 设置消息回调
	node.OnMessage = func(sender string, data []byte) {
		var msg ChatMessage
		json.Unmarshal(data, &msg)
		fmt.Printf("\n📩 [%s]: %s\n> ", sender[:5], msg.Content)
	}

	// 加入房间
	roomName := "movie-night-debug-room"
	// EasyTier 模式下不需要等 DHT，这里只是加入 PubSub
	if err := node.JoinRoom(roomName); err != nil {
		log.Fatalf("加入失败: %v", err)
	}

	fmt.Println("✅ 节点就绪！等待 mDNS 发现或手动连接...")
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" {
			continue
		}
		msg := ChatMessage{Content: text}
		if err := node.Broadcast(msg); err != nil {
			fmt.Printf("❌ 发送失败: %v\n", err)
		} else {
			fmt.Print("> ")
		}
	}
}
