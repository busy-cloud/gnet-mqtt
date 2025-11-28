package types

import (
	"time"
)

// Client 表示一个MQTT客户端连接
type Client struct {
	ClientID     []byte
	CleanSession bool
	KeepAlive    uint16
	Connected    bool
	WillMessage  *WillMessage
	ConnType     string
}

// WillMessage 遗嘱消息
type WillMessage struct {
	Topic   []byte
	Payload []byte
	QoS     byte
	Retain  bool
}

// Subscription 订阅关系
type Subscription struct {
	TopicFilter []byte
	QoS         byte
}

// Message 发布的消息
type Message struct {
	Topic    []byte
	Payload  []byte
	QoS      byte
	Retain   bool
	PacketID uint16
}

// ClientContext 客户端上下文
type ClientContext struct {
	Client     *Client
	LastActive time.Time
	SendChan   chan []byte
}
