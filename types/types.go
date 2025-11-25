package types

// Client表示MQTT客户端连接
type Client struct {
	ClientID     string
	CleanSession bool
	KeepAlive    uint16
	Connected    bool
	WillMessage  *WillMessage
}

// WillMessage遗嘱信息
type WillMessage struct {
	Topic   string
	Payload []byte
	QoS     byte
	Retain  bool
}

// Subscription订阅关系
type Sucscription struct {
	TopicFilter string
	QoS         byte
}

// Message发布的消息
type Message struct {
	Topic    string
	Payload  []byte
	QoS      byte
	Retain   bool
	PacketID uint16
}
