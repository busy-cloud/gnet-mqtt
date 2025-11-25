package broker

import (
	"sync"
	"time"

	"busy-cloud/gnet-mqtt/mqtt"
	"busy-cloud/gnet-mqtt/types"
	"github.com/panjf2000/gnet/v2"
)

// Manager Broker管理器
type Manager struct {
	clients  sync.Map // map[gnet.Conn]*types.Client
	sessions sync.Map // map[string]*types.ClientSession
	router   *Router
	mu       sync.RWMutex
}

// ClientContext 客户端上下文，存储连接相关信息
type ClientContext struct {
	Client     *types.Client
	Conn       gnet.Conn
	LastActive time.Time
}

// NewManager 创建新的管理器
func NewManager() *Manager {
	return &Manager{
		router: NewRouter(),
	}
}

// AddClient 添加客户端
func (m *Manager) AddClient(c gnet.Conn) {
	clientCtx := &ClientContext{
		Client: &types.Client{
			ClientID:  "", // 连接建立时还没有ClientID
			Connected: false,
		},
		Conn:       c,
		LastActive: time.Now(),
	}
	m.clients.Store(c, clientCtx)
}

// RemoveClient 移除客户端
func (m *Manager) RemoveClient(c gnet.Conn) {
	if clientCtx, ok := m.clients.Load(c); ok {
		// 清理路由订阅
		if clientCtx.(*ClientContext).Client.Connected {
			m.router.UnsubscribeAll(clientCtx.(*ClientContext).Client.ClientID)
		}
		m.clients.Delete(c)
	}
}

// GetClient 获取客户端
func (m *Manager) GetClient(c gnet.Conn) (*ClientContext, bool) {
	clientCtx, ok := m.clients.Load(c)
	if !ok {
		return nil, false
	}
	return clientCtx.(*ClientContext), true
}

// HandlePacket 处理MQTT报文
func (m *Manager) HandlePacket(c gnet.Conn, packet interface{}) []byte {
	clientCtx, ok := m.GetClient(c)
	if !ok {
		return nil
	}

	// 更新最后活动时间
	clientCtx.LastActive = time.Now()

	switch p := packet.(type) {
	case *mqtt.ConnectPacket:
		return m.handleConnect(clientCtx, p)
	case *mqtt.PublishPacket:
		return m.handlePublish(clientCtx, p)
	case *mqtt.SubscribePacket:
		return m.handleSubscribe(clientCtx, p)
	case *mqtt.PingReqPacket:
		return m.handlePingReq(clientCtx)
	case *mqtt.DisconnectPacket:
		return m.handleDisconnect(clientCtx)
	default:
		return nil
	}
}

// handleConnect 处理连接请求
func (m *Manager) handleConnect(clientCtx *ClientContext, p *mqtt.ConnectPacket) []byte {
	// 验证协议
	if p.ProtocolName != "MQTT" || p.ProtocolLevel != 4 {
		return mqtt.CreateConnAck(false, 1) // 不支持的协议级别
	}

	// 验证ClientID
	if p.ClientID == "" && !p.CleanSession {
		return mqtt.CreateConnAck(false, 2) // 标识符拒绝
	}

	// 设置客户端信息
	clientCtx.Client.ClientID = p.ClientID
	clientCtx.Client.CleanSession = p.CleanSession
	clientCtx.Client.KeepAlive = p.KeepAlive
	clientCtx.Client.Connected = true

	// 设置遗嘱消息
	if p.WillFlag {
		clientCtx.Client.WillMessage = &types.WillMessage{
			Topic:   p.WillTopic,
			Payload: p.WillMessage,
			QoS:     p.WillQoS,
			Retain:  p.WillRetain,
		}
	}

	// 返回成功CONNACK
	return mqtt.CreateConnAck(false, 0) // 成功
}

// handlePublish 处理发布消息
func (m *Manager) handlePublish(clientCtx *ClientContext, p *mqtt.PublishPacket) []byte {
	// 创建消息
	message := &types.Message{
		Topic:    p.TopicName,
		Payload:  p.Payload,
		QoS:      p.QoS,
		Retain:   p.Retain,
		PacketID: p.PacketID,
	}

	// 路由消息
	m.router.RouteMessage(message)

	// QoS 1需要回复PUBACK
	if p.QoS == 1 {
		return m.createPubAck(p.PacketID)
	}

	return nil
}

// handleSubscribe 处理订阅请求
func (m *Manager) handleSubscribe(clientCtx *ClientContext, p *mqtt.SubscribePacket) []byte {
	returnCodes := make([]byte, len(p.Topics))

	for i, topic := range p.Topics {
		// 添加订阅
		m.router.Subscribe(clientCtx.Client.ClientID, topic.TopicFilter, topic.QoS)
		returnCodes[i] = topic.QoS // 返回授予的QoS
	}

	// 返回SUBACK
	return mqtt.CreateSubAck(p.PacketID, returnCodes)
}

// handlePingReq 处理心跳请求
func (m *Manager) handlePingReq(clientCtx *ClientContext) []byte {
	return mqtt.CreatePingResp()
}

// handleDisconnect 处理断开连接
func (m *Manager) handleDisconnect(clientCtx *ClientContext) []byte {
	clientCtx.Client.Connected = false
	// 连接会在OnClose中清理
	return nil
}

// createPubAck 创建PUBACK包
func (m *Manager) createPubAck(packetID uint16) []byte {
	packetIDBuf := make([]byte, 2)
	packetIDBuf[0] = byte(packetID >> 8)
	packetIDBuf[1] = byte(packetID)

	return mqtt.CreatePacket(mqtt.PUBACK, packetIDBuf)
}

// CheckTimeouts 检查超时连接
func (m *Manager) CheckTimeouts() {
	now := time.Now()

	m.clients.Range(func(key, value interface{}) bool {
		clientCtx := value.(*ClientContext)

		if clientCtx.Client.Connected && clientCtx.Client.KeepAlive > 0 {
			timeout := time.Duration(clientCtx.Client.KeepAlive) * time.Second * 3 / 2
			if now.Sub(clientCtx.LastActive) > timeout {
				// 触发遗嘱消息
				if clientCtx.Client.WillMessage != nil {
					message := &types.Message{
						Topic:   clientCtx.Client.WillMessage.Topic,
						Payload: clientCtx.Client.WillMessage.Payload,
						QoS:     clientCtx.Client.WillMessage.QoS,
						Retain:  clientCtx.Client.WillMessage.Retain,
					}
					m.router.RouteMessage(message)
				}

				// 关闭连接
				_ = clientCtx.Conn.Close()
			}
		}
		return true
	})
}
