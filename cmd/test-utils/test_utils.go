package main

import (
	"fmt"
	"net"
	// "strings"
)

func main() {
	fmt.Println("🚀 开始测试 GetAppIP 工具函数...")
	fmt.Println("--------------------------------------------------")

	// 1. 调用我们编写的函数
	ip, err := GetAppIP()
	if err != nil {
		fmt.Printf("❌ 获取失败: %v\n", err)
	} else {
		fmt.Printf("✅ 成功获取 IP: %s\n", ip)
	}

	fmt.Println("\n📋 系统网卡详细诊断:")
	fmt.Println("--------------------------------------------------")
	
	// 2. 打印所有网卡的详细信息，帮助排查
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, i := range ifaces {
		fmt.Printf("网卡名称: %-10s | MTU: %d | 标志: %s\n", i.Name, i.MTU, i.Flags.String())
		
		// 检查是否有 POINTOPOINT 标志
		isP2P := i.Flags&net.FlagPointToPoint != 0
		isUp := i.Flags&net.FlagUp != 0
		
		addrs, _ := i.Addrs()
		var ipStr string
		for _, addr := range addrs {
			// 简单的 IP 提取逻辑用于展示
			ipStr += addr.String() + " "
		}

		fmt.Printf("   ├─ IP地址: %s\n", ipStr)
		fmt.Printf("   ├─ 状态: UP=%v, P2P=%v\n", isUp, isP2P)

		if isP2P && isUp {
			fmt.Println("   └─ 🎉 [符合条件] 这是一个活动的点对点接口")
		} else {
			fmt.Println("   └─ [跳过] 不符合条件")
		}
		fmt.Println("- - - - - - - - - - - - - - - - - - - -")
	}
}

// ==========================================
// 下面是我们要测试的 utils 逻辑，直接复制过来的
// ==========================================

// GetAppIP 自动探测最合适的监听 IP
func GetAppIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	// 优先寻找带有 POINTOPOINT 标志的网卡
	for _, i := range ifaces {
		// 必须是开启状态 (UP)
		if i.Flags&net.FlagUp == 0 {
			continue
		}

		// 检查是否有 POINTOPOINT 标志
		if i.Flags&net.FlagPointToPoint != 0 {
			ip, err := getIPv4FromInterface(i)
			if err == nil {
				return ip, nil
			}
		}
	}

	return "", fmt.Errorf("no suitable point-to-point interface found")
}

func getIPv4FromInterface(iface net.Interface) (string, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("no ipv4")
}