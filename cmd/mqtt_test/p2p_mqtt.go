package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "time"

    mqtt "github.com/eclipse/paho.mqtt.golang"
    "github.com/pion/ice/v4"
    "github.com/pion/randutil"
    "github.com/pion/stun/v3"
)

type Message struct {
    Type      string `json:"type"`       // "candidate", "auth", "keepalive"
    From      string `json:"from"`       // 发送者 ID
    Ufrag     string `json:"ufrag,omitempty"`
    Pwd       string `json:"pwd,omitempty"`
    Candidate string `json:"candidate,omitempty"`
}

var (
    myID     string
    targetID string
    mqttClient mqtt.Client
    iceAgent   *ice.Agent
)

func main() {
    flag.StringVar(&myID, "id", "", "我的 ID")
    flag.StringVar(&targetID, "peer", "", "对方 ID")
    flag.Parse()

    if myID == "" || targetID == "" {
        fmt.Println("用法: go run p2p_mqtt.go -id=alice -peer=bob")
        os.Exit(1)
    }

    fmt.Printf("🆔 我的 ID: %s\n", myID)
    fmt.Printf("🎯 对方 ID: %s\n\n", targetID)

    // 1️⃣ 连接 MQTT Broker
    fmt.Println("1️⃣ 连接到公共 MQTT Broker...")
    connectMQTT()

    // 2️⃣ 创建 ICE Agent
    fmt.Println("2️⃣ 创建 ICE Agent...")
    createICEAgent()

    // 3️⃣ 订阅对方的消息
    fmt.Printf("3️⃣ 订阅主题: ice/%s\n", myID)
    subscribeTopic()

    // 4️⃣ 发送认证信息
    fmt.Println("4️⃣ 发送认证信息...")
    sendAuth()

    // 5️⃣ 开始收集候选者
    fmt.Println("5️⃣ 收集候选者...\n")
    iceAgent.GatherCandidates()

    // 6️⃣ 等待一段时间收集候选者
    time.Sleep(3 * time.Second)

    // 7️⃣ 建立连接
    fmt.Println("\n6️⃣ 尝试建立 P2P 连接...")
    conn := establishConnection()

    if conn == nil {
        fmt.Println("❌ 连接失败")
        return
    }

    fmt.Println("✅ 连接成功！\n")

    // 8️⃣ 启动保活和端口监控
    go keepalive(conn)
    go monitorConnection(conn)

    // 9️⃣ 数据传输
    go sendLoop(conn)
    receiveLoop(conn)
}

// 连接 MQTT
func connectMQTT() {
    opts := mqtt.NewClientOptions()
    opts.AddBroker("tcp://broker.emqx.io:1883")
    opts.SetClientID(fmt.Sprintf("ice-client-%s-%d", myID, time.Now().Unix()))
    opts.SetKeepAlive(30 * time.Second)
    opts.SetPingTimeout(10 * time.Second)

    opts.OnConnect = func(c mqtt.Client) {
        fmt.Println("   ✅ MQTT 连接成功")
    }

    opts.OnConnectionLost = func(c mqtt.Client, err error) {
        fmt.Printf("   ❌ MQTT 连接丢失: %v\n", err)
    }

    mqttClient = mqtt.NewClient(opts)
    if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
        panic(token.Error())
    }
}

// 创建 ICE Agent
func createICEAgent() {
    var err error
    iceAgent, err = ice.NewAgentWithOptions(
        ice.WithNetworkTypes([]ice.NetworkType{ice.NetworkTypeUDP4}),
		ice.WithUrls([]*stun.URI{
			{
				Scheme: stun.SchemeTypeSTUN,
				Host:   "stun.miwifi.com",
				Port:   3478,
				Proto:  stun.ProtoTypeUDP,
			},
		}),
    )
    if err != nil {
        panic(err)
    }

    // 候选者回调 - 通过 MQTT 实时发送
    iceAgent.OnCandidate(func(c ice.Candidate) {
        if c == nil {
            fmt.Println("   ✅ 候选者收集完成")
            return
        }

        fmt.Printf("   📤 发现候选者: %s:%d -> 发送给 %s\n", 
            c.Address(), c.Port(), targetID)

        // 实时通过 MQTT 发送候选者
        msg := Message{
            Type:      "candidate",
            From:      myID,
            Candidate: c.Marshal(),
        }
        publishMessage(msg)
    })

    // 连接状态变化
    iceAgent.OnConnectionStateChange(func(state ice.ConnectionState) {
        fmt.Printf("   📡 ICE 状态: %s\n", state)
    })
}

// 订阅 MQTT 主题
func subscribeTopic() {
    topic := fmt.Sprintf("ice/%s", myID)
    
    token := mqttClient.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
        var message Message
        if err := json.Unmarshal(msg.Payload(), &message); err != nil {
            return
        }

        handleMessage(message)
    })

    token.Wait()
    if token.Error() != nil {
        panic(token.Error())
    }
}

// 处理收到的消息
func handleMessage(msg Message) {
    switch msg.Type {
    case "auth":
        fmt.Printf("   📥 收到认证: ufrag=%s\n", msg.Ufrag)
        // 稍后用于连接

    case "candidate":
        c, err := ice.UnmarshalCandidate(msg.Candidate)
        if err != nil {
            return
        }
        fmt.Printf("   📥 收到候选者: %s:%d\n", c.Address(), c.Port())
        iceAgent.AddRemoteCandidate(c)

    case "keepalive":
        // 保活消息
        fmt.Printf("   💓 收到保活消息来自 %s\n", msg.From)
    }
}

// 发送认证信息
func sendAuth() {
    ufrag, pwd, _ := iceAgent.GetLocalUserCredentials()
    
    msg := Message{
        Type:  "auth",
        From:  myID,
        Ufrag: ufrag,
        Pwd:   pwd,
    }

    publishMessage(msg)
    fmt.Printf("   ✅ 已发送: ufrag=%s\n", ufrag)
}

// 发布消息到 MQTT
func publishMessage(msg Message) {
    topic := fmt.Sprintf("ice/%s", targetID)
    
    data, _ := json.Marshal(msg)
    token := mqttClient.Publish(topic, 0, false, data)
    token.Wait()
}

// 建立连接（简化版，实际需要等待对方认证信息）
func establishConnection() *ice.Conn {
    // 等待接收对方认证信息
    time.Sleep(2 * time.Second)
    
    // 这里应该从接收到的消息中获取，简化起见先跳过
    // 实际使用需要存储接收到的 auth 消息
    
    remoteUfrag := "temp" // 实际应该从消息中获取
    remotePwd := "temp"

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // 根据 ID 字符串大小决定谁是 controlling
    var conn *ice.Conn
    var err error

    if myID < targetID {
        fmt.Println("   角色: Controlling")
        conn, err = iceAgent.Dial(ctx, remoteUfrag, remotePwd)
    } else {
        fmt.Println("   角色: Controlled")
        conn, err = iceAgent.Accept(ctx, remoteUfrag, remotePwd)
    }

    if err != nil {
        fmt.Printf("   ❌ 连接失败: %v\n", err)
        return nil
    }

    return conn
}

// 保活 - 定期通过 MQTT 发送心跳
func keepalive(conn *ice.Conn) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        msg := Message{
            Type: "keepalive",
            From: myID,
        }
        publishMessage(msg)
        
        // 同时通过 ICE 连接发送保活
        conn.Write([]byte("ping"))
    }
}

// 监控连接状态
func monitorConnection(conn *ice.Conn) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        // 检查候选者对
        if pair, err := iceAgent.GetSelectedCandidatePair(); err == nil && pair != nil {
            fmt.Printf("   🔗 当前连接: %s:%d ↔ %s:%d\n",
                pair.Local.Address(), pair.Local.Port(),
                pair.Remote.Address(), pair.Remote.Port())
        }
    }
}

// 发送数据循环
func sendLoop(conn *ice.Conn) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        msg, _ := randutil.GenerateCryptoRandomString(10, "abcdefghijklmnopqrstuvwxyz")
        conn.Write([]byte(msg))
        fmt.Printf("📤 发送: %s\n", msg)
    }
}

// 接收数据循环
func receiveLoop(conn *ice.Conn) {
    buf := make([]byte, 1500)
    for {
        n, err := conn.Read(buf)
        if err != nil {
            fmt.Printf("❌ 读取错误: %v\n", err)
            return
        }
        fmt.Printf("📥 接收: %s\n", string(buf[:n]))
    }
}