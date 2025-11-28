package network

import (
	"log/slog"

	"busy-cloud/gnet-mqtt/broker"
	"busy-cloud/gnet-mqtt/mqtt"
	"busy-cloud/gnet-mqtt/types"
)

// MQTTConnectionHandler MQTT连接处理器
type MQTTConnectionHandler struct {
	broker *broker.Manager
	logger *slog.Logger
}

// NewMQTTConnectionHandler 创建新的MQTT连接处理器
func NewMQTTConnectionHandler(broker *broker.Manager, logger *slog.Logger) *MQTTConnectionHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &MQTTConnectionHandler{
		broker: broker,
		logger: logger,
	}
}

// OnOpen 处理新连接
func (h *MQTTConnectionHandler) OnOpen(conn types.Conn) {
	connType := "tcp"
	h.broker.AddClient(conn, connType)
}

// OnMessage 处理接收到的消息
func (h *MQTTConnectionHandler) OnMessage(conn types.Conn, data []byte) {
	// 解析MQTT报文
	packet, err := mqtt.DecodePacket(data)
	if err != nil {
		h.logger.Error("Failed to parse MQTT packet", "error", err)
		conn.Close()
		return
	}

	// 处理报文
	h.broker.HandlePacket(conn, packet)
}

// OnClose 处理连接关闭
func (h *MQTTConnectionHandler) OnClose(conn types.Conn, err error) {
	h.broker.RemoveClient(conn)
}
