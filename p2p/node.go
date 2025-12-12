package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
)

// 定义局域网发现暗号
const DiscoveryTag = "movie-night-test-v1"

type P2PNode struct {
	Host      host.Host
	PubSub    *pubsub.PubSub
	Topic     *pubsub.Topic
	Sub       *pubsub.Subscription
	ctx       context.Context
	OnMessage func(sender string, data []byte)
}

// 预定义一些公共的稳定节点作为中继（这里使用的是 IPFS 官方引导节点）
// 注意：生产环境中建议搭建自己的 dCircuit Relay v2 节点
var DefaultStaticRelays = []string{
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjXDcoAYUf4",
}

func NewNode(ctx context.Context, listenIP string) (*P2PNode, error) {
	if listenIP == "" {
		listenIP = "0.0.0.0"
	}

	// 1. 准备静态中继节点列表
	var staticRelays []peer.AddrInfo
	for _, s := range DefaultStaticRelays {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			continue
		}
		pi, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			continue
		}
		staticRelays = append(staticRelays, *pi)
	}

	// 2. 创建 Host (极简配置，移除 DHT)
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/%s/tcp/0", listenIP),
			fmt.Sprintf("/ip4/%s/udp/0/quic-v1", listenIP),
		),
		libp2p.EnableRelay(), // 允许使用中继
		// 关键点：使用静态中继列表，不再去 DHT 搜寻中继
		libp2p.EnableAutoRelayWithStaticRelays(staticRelays),
		libp2p.EnableHolePunching(), // 开启 NAT 打洞
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create host: %w", err)
	}

	fmt.Printf("🆔 [P2P] Host ID: %s\n", h.ID())

	// 3. 连接到静态中继节点 (这一步是必须的，否则 AutoRelay 没法工作)
	// 虽然 EnableAutoRelayWithStaticRelays 会尝试连接，但显式连接更稳妥
	var wg sync.WaitGroup
	fmt.Println("⏳ [Relay] Connecting to static relays...")
	for _, relay := range staticRelays {
		wg.Add(1)
		go func(pi peer.AddrInfo) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second) // 快速超时，不卡启动
			defer cancel()
			h.Connect(ctx, pi)
		}(relay)
	}
	wg.Wait()
	fmt.Printf("✅ [Relay] Connected to relays (Ready for Hole Punching)\n")

	// 4. 初始化 PubSub
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("create pubsub failed: %w", err)
	}

	node := &P2PNode{
		Host:   h,
		PubSub: ps,
		ctx:    ctx,
	}

	// 5. 启动 mDNS (这是这套方案中唯一的自动发现机制)
	// 在没有 DHT 的情况下，如果不在局域网，只能靠手动复制粘贴地址连接
	if err := setupMDNS(h, DiscoveryTag); err != nil {
		fmt.Printf("⚠️ mDNS setup failed: %v\n", err)
	} else {
		fmt.Println("📡 [Discovery] mDNS (LAN) enabled")
	}

	return node, nil
}

// mDNS 处理器 (保持不变)
type mdnsNotifee struct {
	h host.Host
}

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if pi.ID == n.h.ID() {
		return
	}
	// 局域网内发现节点，直接连接
	n.h.Connect(context.Background(), pi)
}

func setupMDNS(h host.Host, serviceName string) error {
	mn := &mdnsNotifee{h: h}
	s := mdns.NewMdnsService(h, serviceName, mn)
	return s.Start()
}

// JoinRoom, Broadcast, readLoop 逻辑完全不需要变，省略...
func (n *P2PNode) JoinRoom(roomName string) error {
	topic, err := n.PubSub.Join(roomName)
	if err != nil {
		return err
	}
	sub, err := topic.Subscribe()
	if err != nil {
		return err
	}
	n.Topic = topic
	n.Sub = sub
	go n.readLoop()
	return nil
}

func (n *P2PNode) Broadcast(data interface{}) error {
	if n.Topic == nil {
		return fmt.Errorf("not joined any room")
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	// 调试用：打印当前已知的连接数
	// peers := n.Topic.ListPeers()
	// fmt.Printf("DEBUG: Broadcasting to %d peers\n", len(peers))
	return n.Topic.Publish(n.ctx, bytes)
}

func (n *P2PNode) readLoop() {
	for {
		msg, err := n.Sub.Next(n.ctx)
		if err != nil {
			return
		}
		if msg.ReceivedFrom == n.Host.ID() {
			continue
		}
		if n.OnMessage != nil {
			n.OnMessage(msg.ReceivedFrom.String(), msg.Data)
		}
	}
}

func (n *P2PNode) PrintMyAddresses() {
	fmt.Println("📋 本机监听地址 (复制给同局域网设备):")
	fmt.Println("---------------------------------------------------------")
	
    // 打印当前所有地址
	for _, addr := range n.Host.Addrs() {
		fullAddr := fmt.Sprintf("%s/p2p/%s", addr, n.Host.ID())
		fmt.Println(fullAddr)
	}
	fmt.Println("---------------------------------------------------------")
    
    // 如果没有公网地址，提示一下
    fmt.Println("💡 提示: 如果你是跨互联网连接，请等待 10-20 秒，")
    fmt.Println("        直到看到包含 /p2p-circuit/ 的中继地址或公网 IP 出现。")
}

// ConnectTo 手动连接到指定节点
// targetAddrStr 格式如: /ip4/127.0.0.1/tcp/44209/p2p/12D3Koo...
func (n *P2PNode) ConnectTo(targetAddrStr string) error {
	// 1. 解析字符串为 Multiaddr 对象
	maddr, err := multiaddr.NewMultiaddr(targetAddrStr)
	if err != nil {
		return fmt.Errorf("地址格式错误: %w", err)
	}

	// 2. 从 Multiaddr 中提取 Peer 信息 (ID 和 地址)
	peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("无法提取节点信息: %w", err)
	}

	// 3. 建立连接
	ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
	defer cancel()

	fmt.Printf("⏳ 正在尝试连接到: %s ...\n", peerInfo.ID)
	if err := n.Host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	fmt.Printf("🔗 成功连接到节点: %s\n", peerInfo.ID)
	return nil
}