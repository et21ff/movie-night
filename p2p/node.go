package p2p

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// P2PNode 封装 P2P 逻辑
type P2PNode struct {
	Host      host.Host
	PubSub    *pubsub.PubSub
	Topic     *pubsub.Topic
	Sub       *pubsub.Subscription
	ctx       context.Context
	OnMessage func(sender string, data []byte)
}

// NewNode 创建节点
func NewNode(ctx context.Context) (*P2PNode, error) {
	// 1. 创建 Host
	// 监听所有网卡的随机端口 (0.0.0.0)
	// 启用 TCP 和 UDP (QUIC) 以获得最佳穿透性能
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0", "/ip4/0.0.0.0/udp/0/quic-v1"),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 Host 失败: %w", err)
	}

	fmt.Printf("🆔 [P2P] 节点启动: %s\n", h.ID())

	// 2. 启动 mDNS (局域网/EasyTier 自动发现)
	// ServiceTag 必须一致，两台电脑才能互相看见
	mdnsService := mdns.NewMdnsService(h, "movie-night-lan", &discoveryNotifee{h: h})
	if err := mdnsService.Start(); err != nil {
		fmt.Println("⚠️ mDNS 启动失败:", err)
	} else {
		fmt.Println("⚡ [Discovery] mDNS 广播已启动，正在寻找局域网队友...")
	}

	// 3. 创建 PubSub (聊天/同步)
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("创建 PubSub 失败: %w", err)
	}

	return &P2PNode{
		Host:   h,
		PubSub: ps,
		ctx:    ctx,
	}, nil
}

// JoinRoom 加入房间
func (n *P2PNode) JoinRoom(roomName string) error {
	// 加入 PubSub 频道
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

	// 启动接收循环
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

// 内部消息读取循环
func (n *P2PNode) readLoop() {
	for {
		msg, err := n.Sub.Next(n.ctx)
		if err != nil {
			return
		}
		// 忽略自己发的消息
		if msg.ReceivedFrom == n.Host.ID() {
			continue
		}
		if n.OnMessage != nil {
			n.OnMessage(msg.ReceivedFrom.String(), msg.Data)
		}
	}
}

// ---------------- mDNS 回调逻辑 ----------------

type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound 当 mDNS 发现邻居时触发
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if pi.ID == n.h.ID() {
		return
	}
	// 这里不打印日志了，避免刷屏，默默连接即可
	// 连接是幂等的，多次连接没关系
	go n.h.Connect(context.Background(), pi)
}
