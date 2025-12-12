package p2p

import (
	"fmt"
	"net"
)

// GetAppIP 自动探测最合适的监听 IP
// 优先寻找带有 POINTOPOINT 标志的网卡 (如 EasyTier/VPN tun设备)
func GetAppIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	// 1. 第一轮遍历：寻找 POINTOPOINT 类型的网卡 (EasyTier 的特征)
	for _, i := range ifaces {
		// 必须是开启状态 (UP)
		if i.Flags&net.FlagUp == 0 {
			continue
		}

		// 关键点：检查是否有 POINTOPOINT 标志
		// EasyTier 的 tun0 通常具备这个标志
		if i.Flags&net.FlagPointToPoint != 0 {
			ip, err := getIPv4FromInterface(i)
			if err == nil {
				// 找到了！直接返回
				// fmt.Printf("🕵️ 发现 P2P 接口: %s\n", i.Name)
				return ip, nil
			}
		}
	}

	// 2. (可选) 第二轮遍历：如果没找到 P2P 网卡，尝试找名字包含 "tun" 或 "easy" 的
	// 这一步是为了兼容某些系统可能没正确设置 Flags 的情况
	// for _, i := range ifaces {
	// 	if i.Flags&net.FlagUp == 0 { continue }
	// 	if strings.Contains(i.Name, "tun") || strings.Contains(i.Name, "easy") {
	// 		ip, err := getIPv4FromInterface(i)
	// 		if err == nil { return ip, nil }
	// 	}
	// }

	return "", fmt.Errorf("no suitable point-to-point interface found")
}

// 辅助函数：从网卡中提取 IPv4
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
		// 只取 IPv4，且不是 127.0.0.1
		if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("no ipv4")
}