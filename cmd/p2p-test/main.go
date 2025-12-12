package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"movie-night/p2p" // 替换为你的实际包名
)

func main() {
	ctx := context.Background()

	// 1. 创建节点 (使用简化版 NewNode)
	node, err := p2p.NewNode(ctx, "10.126.126.2")
	if err != nil {
		panic(err)
	}

	// 在 main 函数中，启动节点后加入：
go func() {
    ticker := time.NewTicker(5 * time.Second)
    seen := make(map[string]bool)
    for range ticker.C {
        // 检查有没有新地址（例如公网IP或中继地址）出现
        for _, addr := range node.Host.Addrs() {
            s := addr.String()
            if !seen[s] {
                // 过滤掉本地回环，只看有意义的
                if !strings.Contains(s, "127.0.0.1") {
                    fmt.Printf("\n🆕 发现新地址 (可能是公网/中继): %s/p2p/%s\n> ", s, node.Host.ID())
                }
                seen[s] = true
            }
        }
    }
}()

	// 2. 设置消息回调
	node.OnMessage = func(sender string, data []byte) {
		fmt.Printf("\n📩 收到来自 [%s] 的消息: %s\n> ", sender[:5], string(data))
	}

	// 3. 加入房间
	node.JoinRoom("movie-night-room")

	// 4. 打印我的地址，供别人连接
	node.PrintMyAddresses()

	// 5. 简单的命令行交互
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("💡 输入 '/connect <地址>' 来连接对方，或者直接输入消息发送")
	fmt.Print("> ")

	

	for scanner.Scan() {
		text := scanner.Text()
		
		// 处理连接命令
		if strings.HasPrefix(text, "/connect ") {
			addr := strings.TrimPrefix(text, "/connect ")
			addr = strings.TrimSpace(addr) // 去除可能的空格
			if err := node.ConnectTo(addr); err != nil {
				fmt.Printf("❌ 连接错误: %v\n", err)
			}
			fmt.Print("> ")
			continue
		}

		// 处理发送消息
		if text != "" {
			if err := node.Broadcast(map[string]string{"msg": text}); err != nil {
				fmt.Println("❌ 发送失败:", err)
			}
		}
		fmt.Print("> ")
	}
}