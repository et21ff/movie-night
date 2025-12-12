package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// 常量定义：固定端口，方便在 EasyTier 网络中互相寻找
const (
	P2PPort       = 12111 // TCP/UDP 数据传输端口
	DiscoveryPort = 12112 // UDP 自动发现广播端口
)

// P2PNode 结构体：相当于面向对象中的 "类"，保存节点的状态
type P2PNode struct {
	Host   host.Host            // LibP2P 的主机对象，代表你自己
	PubSub *pubsub.PubSub       // 发布订阅系统管理器
	Topic  *pubsub.Topic        // 当前加入的聊天室/频道
	Sub    *pubsub.Subscription // 消息订阅句柄，用于接收消息
	ctx    context.Context      // 上下文，用于控制程序的生命周期（退出、超时）

	// 回调函数：当收到消息时，调用这个函数通知上层 (main.go)
	OnMessage func(sender string, data []byte)
}

// DiscoveryPacket 结构体：定义广播包的 JSON 格式
// `json:"..."` 是 Go 的 Struct Tag，告诉 JSON 库序列化时字段叫什么名字
type DiscoveryPacket struct {
	PeerID string   `json:"peer_id"` // 我的 ID
	Addrs  []string `json:"addrs"`   // 我的地址列表
}

// NewNode 构造函数：创建一个新的 P2P 节点
// 这里的 listenIP 是从 main.go 传进来的 EasyTier IP
func NewNode(ctx context.Context, listenIP string) (*P2PNode, error) {
	if listenIP == "" {
		listenIP = "0.0.0.0"
	}

	// 1. 创建 Host (LibP2P 的核心)
	// libp2p.New 使用了 "Functional Options" 模式（Go 常用设计模式）
	h, err := libp2p.New(
		// 监听地址：同时支持 TCP 和 QUIC (UDP)
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/%s/tcp/%d", listenIP, P2PPort),
			fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", listenIP, P2PPort),
		),
		// 开启 NAT 打洞和中继支持（虽然在 EasyTier 里可能用不上，但加上无害）
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 Host 失败: %w", err)
	}

	fmt.Printf("🆔 [P2P] 节点启动 ID: %s\n", h.ID())

	// 2. 创建 GossipSub (一种高效的消息广播协议)
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("创建 PubSub 失败: %w", err)
	}

	// 3. 初始化节点结构体
	node := &P2PNode{
		Host:   h,
		PubSub: ps,
		ctx:    ctx,
	}

	h.Network().Notify(&netNotifiee{})
	// 4. 启动后台协程 (Goroutine)：处理 UDP 广播发现
	// `go` 关键字意味着这行代码会立即返回，startDiscovery 在后台并发运行
	go node.startDiscovery(listenIP)

	return node, nil
}

// startDiscovery 启动发现逻辑：一边听，一边喊
func (n *P2PNode) startDiscovery(bindIP string) {
	fmt.Printf("📡 [Discovery] 启动自动发现 (UDP %d)...\n", DiscoveryPort)

	// 启动接收协程
	go n.listenBroadcast()

	// 启动发送协程 (如果绑定的是具体 IP)
	if bindIP != "0.0.0.0" {
		go n.sendBroadcast(bindIP)
	}
}

// sendBroadcast 发送广播：我是谁，我在哪
func (n *P2PNode) sendBroadcast(localIP string) {
	// 目标地址：255.255.255.255 代表全网广播
	dstAddr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("255.255.255.255:%d", DiscoveryPort))

	// 源地址：必须绑定到 EasyTier 的 IP，否则包可能从物理网卡跑出去
	srcAddr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:0", localIP)) // :0 表示系统随机分配一个空闲端口

	conn, err := net.DialUDP("udp4", srcAddr, dstAddr)
	if err != nil {
		fmt.Printf("❌ 广播发送失败: %v\n", err)
		return
	}
	// defer 关键字：确保函数退出时关闭连接，防止资源泄露
	defer conn.Close()

	// 定时器：每 3 秒触发一次
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		// select 语句：Go 的多路复用，用于处理多个 Channel
		select {
		case <-n.ctx.Done(): // 如果主程序退出了
			return
		case <-ticker.C: // 如果定时器时间到了
			// 准备数据包
			packet := DiscoveryPacket{PeerID: n.Host.ID().String()}
			for _, addr := range n.Host.Addrs() {
				// 拼接完整地址：/ip4/10.x.x.x/tcp/12111/p2p/Qm...
				packet.Addrs = append(packet.Addrs, fmt.Sprintf("%s/p2p/%s", addr, n.Host.ID()))
			}
			// 序列化为 JSON
			data, _ := json.Marshal(packet)
			conn.Write(data)
		}
	}
}

// listenBroadcast 接收广播：发现新邻居
func (n *P2PNode) listenBroadcast() {
	// 监听所有网卡 (0.0.0.0) 的 UDP 端口
	addr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("0.0.0.0:%d", DiscoveryPort))
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		fmt.Printf("❌ 广播监听失败: %v\n", err)
		return
	}
	defer conn.Close()

	buf := make([]byte, 4096)
	for {
		// 读取 UDP 数据包 (阻塞操作)
		count, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if n.ctx.Err() != nil {
				return
			} // 正常退出
			continue
		}

		// 解析 JSON
		var packet DiscoveryPacket
		if err := json.Unmarshal(buf[:count], &packet); err != nil {
			continue
		}

		// 过滤掉自己发出的包
		if packet.PeerID == n.Host.ID().String() {
			continue
		}

		// 检查是否已经是连接状态
		targetID, err := peer.Decode(packet.PeerID)
		if err != nil {
			continue
		}

		if n.Host.Network().Connectedness(targetID) == network.Connected {
			continue // 已经连上了，忽略
		}

		fmt.Printf("🔭 [Discovery] 发现新节点: %s\n", packet.PeerID)

		// 开启一个临时协程去连接，防止阻塞接收循环
		go n.connectToPeer(packet)
	}
}

func (n *P2PNode) connectToPeer(packet DiscoveryPacket) {
	for _, addrStr := range packet.Addrs {
		// 解析多格式地址 (Multiaddr)
		ma, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}

		pi, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			continue
		}

		// 设置 5 秒连接超时
		ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
		if err := n.Host.Connect(ctx, *pi); err == nil {
			fmt.Printf("✅ [Discovery] 已自动连接到: %s\n", packet.PeerID)
			cancel() // 成功后取消超时上下文，释放资源
			return
		}
		cancel()
	}
}

// JoinRoom 加入房间 (PubSub)
func (n *P2PNode) JoinRoom(roomName string) error {
	// Join: 告诉网络我对这个话题感兴趣
	topic, err := n.PubSub.Join(roomName)
	if err != nil {
		return err
	}

	// Subscribe: 开始接收数据
	sub, err := topic.Subscribe()
	if err != nil {
		return err
	}

	n.Topic = topic
	n.Sub = sub

	// 启动后台读取消息循环
	go n.readLoop()
	return nil
}

// Broadcast 广播消息
func (n *P2PNode) Broadcast(data interface{}) error {
	if n.Topic == nil {
		return fmt.Errorf("未加入房间")
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return n.Topic.Publish(n.ctx, bytes)
}

// readLoop 持续读取消息
func (n *P2PNode) readLoop() {
	for {
		msg, err := n.Sub.Next(n.ctx) // 阻塞等待下一条消息
		if err != nil {
			return
		} // 比如 context 被取消，这里会报错退出

		// 忽略自己发的消息
		if msg.ReceivedFrom == n.Host.ID() {
			continue
		}

		// 回调通知 main.go
		if n.OnMessage != nil {
			n.OnMessage(msg.ReceivedFrom.String(), msg.Data)
		}
	}
}

// netNotifiee 实现 network.Notifiee 接口，用于监听底层连接事件
type netNotifiee struct{}

// 当有新连接建立时（无论是主动还是被动）触发
func (n *netNotifiee) Connected(net network.Network, conn network.Conn) {
	fmt.Printf("🤝 [Network] 连接建立: %s (方向: %s)\n",
		conn.RemotePeer().String()[:10]+"...", // 只打印 ID 前10位
		conn.Stat().Direction.String(),        // 打印是 Inbound(被动) 还是 Outbound(主动)
	)
}

// 当连接断开时触发
func (n *netNotifiee) Disconnected(net network.Network, conn network.Conn) {
	fmt.Printf("👋 [Network] 连接断开: %s\n", conn.RemotePeer().String()[:10]+"...")
}

// 下面这些接口必须实现，但我们可以留空
func (n *netNotifiee) Listen(network.Network, multiaddr.Multiaddr)      {}
func (n *netNotifiee) ListenClose(network.Network, multiaddr.Multiaddr) {}
func (n *netNotifiee) OpenedStream(network.Network, network.Stream)     {}
func (n *netNotifiee) ClosedStream(network.Network, network.Stream)     {}
