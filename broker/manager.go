package broker

import (
	"log/slog"
	"sync"
	"time"

	"busy-cloud/gnet-mqtt/mqtt"
	"busy-cloud/gnet-mqtt/types"
)

// Manager Broker管理器
type Manager struct {
	clients  sync.Map // map[types.Conn]*ClientContext
	sessions sync.Map // map[string]*ClientSession
	router   *Router
	mu       sync.RWMutex
	logger   *slog.Logger
}

// ClientContext 客户端上下文
type ClientContext struct {
	Client     *types.Client
	LastActive time.Time
	SendChan   chan []byte
}

// NewManager 创建新的管理器
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		router: NewRouter(logger),
		logger: logger,
	}
}

// AddClient 添加客户端
func (m *Manager) AddClient(conn types.Conn, connType string) {
	clientCtx := &ClientContext{
		Client: &types.Client{
			ClientID:  []byte{}, // 空字节数组
			Connected: false,
			ConnType:  connType,
		},
		LastActive: time.Now(),
		SendChan:   make(chan []byte, 100),
	}
	m.clients.Store(conn, clientCtx)

	// 启动发送协程
	go m.sendLoop(conn, clientCtx)

	m.logger.Info("New client connected",
		"remote_addr", conn.RemoteAddr().String(),
		"conn_type", connType)
}

// RemoveClient 移除客户端
func (m *Manager) RemoveClient(conn types.Conn) {
	if clientCtx, ok := m.clients.Load(conn); ok {
		close(clientCtx.(*ClientContext).SendChan)

		if clientCtx.(*ClientContext).Client.Connected {
			clientID := string(clientCtx.(*ClientContext).Client.ClientID)
			m.router.UnsubscribeAll(clientID)

			// 发布遗嘱消息
			if clientCtx.(*ClientContext).Client.WillMessage != nil {
				message := &types.Message{
					Topic:   clientCtx.(*ClientContext).Client.WillMessage.Topic,
					Payload: clientCtx.(*ClientContext).Client.WillMessage.Payload,
					QoS:     clientCtx.(*ClientContext).Client.WillMessage.QoS,
					Retain:  clientCtx.(*ClientContext).Client.WillMessage.Retain,
				}
				m.router.RouteMessage(message)
			}
		}

		m.clients.Delete(conn)
		m.logger.Info("Client disconnected",
			"remote_addr", conn.RemoteAddr().String())
	}
}

// sendLoop 发送消息循环
func (m *Manager) sendLoop(conn types.Conn, clientCtx *ClientContext) {
	for data := range clientCtx.SendChan {
		_, err := conn.Write(data)
		if err != nil {
			m.logger.Error("Failed to send data to client",
				"error", err,
				"remote_addr", conn.RemoteAddr().String())
			break
		}
	}
}

// HandlePacket 处理MQTT报文
func (m *Manager) HandlePacket(conn types.Conn, packet interface{}) {
	clientCtx, ok := m.clients.Load(conn)
	if !ok {
		return
	}

	clientCtx.(*ClientContext).LastActive = time.Now()

	var response []byte
	switch p := packet.(type) {
	case *mqtt.ConnectPacket:
		response = m.handleConnect(clientCtx.(*ClientContext), p)
	case *mqtt.PublishPacket:
		response = m.handlePublish(clientCtx.(*ClientContext), p)
	case *mqtt.SubscribePacket:
		response = m.handleSubscribe(clientCtx.(*ClientContext), p)
	case *mqtt.PingReqPacket:
		response = m.handlePingReq(clientCtx.(*ClientContext))
	case *mqtt.DisconnectPacket:
		m.handleDisconnect(clientCtx.(*ClientContext))
	default:
		m.logger.Warn("Unknown packet type received")
	}

	if response != nil {
		select {
		case clientCtx.(*ClientContext).SendChan <- response:
		default:
			m.logger.Warn("Send channel full, dropping packet")
		}
	}
}

// handleConnect 处理连接请求
func (m *Manager) handleConnect(clientCtx *ClientContext, p *mqtt.ConnectPacket) []byte {
	// 验证协议
	if string(p.ProtocolName) != "MQTT" || p.ProtocolLevel != 4 {
		return mqtt.CreateConnAck(false, 1)
	}

	// 验证ClientID
	if len(p.ClientID) == 0 && !p.CleanSession {
		return mqtt.CreateConnAck(false, 2)
	}

	// 设置客户端信息 - 直接使用字节数组
	clientCtx.Client.ClientID = p.ClientID
	clientCtx.Client.CleanSession = p.CleanSession
	clientCtx.Client.KeepAlive = p.KeepAlive
	clientCtx.Client.Connected = true

	// 设置遗嘱消息 - 直接使用字节数组
	if p.WillFlag {
		clientCtx.Client.WillMessage = &types.WillMessage{
			Topic:   p.WillTopic,
			Payload: p.WillMessage,
			QoS:     p.WillQoS,
			Retain:  p.WillRetain,
		}
	}

	m.logger.Info("Client connected successfully",
		"client_id", string(p.ClientID),
		"clean_session", p.CleanSession)

	return mqtt.CreateConnAck(false, 0)
}

// handlePublish 处理发布消息
func (m *Manager) handlePublish(clientCtx *ClientContext, p *mqtt.PublishPacket) []byte {
	message := &types.Message{
		Topic:    p.TopicName,
		Payload:  p.Payload,
		QoS:      p.QoS,
		Retain:   p.Retain,
		PacketID: p.PacketID,
	}

	m.router.RouteMessage(message)

	m.logger.Debug("Message published",
		"topic", string(p.TopicName),
		"qos", p.QoS,
		"payload_size", len(p.Payload))

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
		m.router.Subscribe(string(clientCtx.Client.ClientID), topic.TopicFilter, topic.QoS)
		returnCodes[i] = topic.QoS

		m.logger.Debug("Client subscribed",
			"client_id", string(clientCtx.Client.ClientID),
			"topic", string(topic.TopicFilter),
			"qos", topic.QoS)
	}

	return mqtt.CreateSubAck(p.PacketID, returnCodes)
}

// handlePingReq 处理心跳请求
func (m *Manager) handlePingReq(clientCtx *ClientContext) []byte {
	return mqtt.CreatePingResp()
}

// handleDisconnect 处理断开连接
func (m *Manager) handleDisconnect(clientCtx *ClientContext) []byte {
	clientCtx.Client.Connected = false
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
		conn := key.(types.Conn)
		clientCtx := value.(*ClientContext)

		if clientCtx.Client.Connected && clientCtx.Client.KeepAlive > 0 {
			timeout := time.Duration(clientCtx.Client.KeepAlive) * time.Second * 3 / 2
			if now.Sub(clientCtx.LastActive) > timeout {
				m.logger.Warn("Client timeout, disconnecting",
					"client_id", string(clientCtx.Client.ClientID),
					"remote_addr", conn.RemoteAddr().String())

				conn.Close()
			}
		}
		return true
	})
}
