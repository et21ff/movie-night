package main

import (
	"fmt"
	"net"
)

func main() {
	// 监听 12112 端口
	port := 12112
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		fmt.Printf("❌ 监听失败: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("🎧 [Go接收端] 正在监听 UDP %d ...\n", port)

	buf := make([]byte, 1024)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Read Error:", err)
			continue
		}
		fmt.Printf("📩 收到来自 [%s] 的数据: %s\n", remote, string(buf[:n]))
	}
}
