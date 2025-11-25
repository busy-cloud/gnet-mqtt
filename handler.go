package main

import (
	"time"

	"busy-cloud/gnet-mqtt/broker"
	"busy-cloud/gnet-mqtt/mqtt"
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

type Handler struct {
	eng    gnet.Engine
	broker *broker.Manager
	codec  mqtt.MQTTCodec
}

func (h *Handler) OnBoot(eng gnet.Engine) (action gnet.Action) {
	h.eng = eng
	h.broker = broker.NewManager()
	logging.Infof("MQTT Broker started on :1883")
	return gnet.None
}

func (h *Handler) OnShutdown(eng gnet.Engine) {
	logging.Infof("MQTT Broker is shutting down")
}

func (h *Handler) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	h.broker.AddClient(c)
	logging.Infof("New client connected: %s", c.RemoteAddr().String())
	return nil, gnet.None
}

func (h *Handler) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	h.broker.RemoveClient(c)
	if err != nil {
		logging.Infof("Client disconnected: %s, error: %v", c.RemoteAddr().String(), err)
	} else {
		logging.Infof("Client disconnected: %s", c.RemoteAddr().String())
	}
	return gnet.None
}

func (h *Handler) OnTraffic(c gnet.Conn) (action gnet.Action) {
	// 使用编解码器处理数据
	packetData, err := h.codec.Decode(c)
	if err != nil {
		logging.Errorf("Failed to decode packet: %v", err)
		return gnet.Close
	}

	if packetData == nil {
		// 数据不完整，等待更多数据
		return gnet.None
	}

	// 解析MQTT报文
	packet, err := mqtt.DecodePacket(packetData)
	if err != nil {
		logging.Errorf("Failed to parse MQTT packet: %v", err)
		return gnet.Close
	}

	// 处理报文并获取响应
	response := h.broker.HandlePacket(c, packet)
	if response != nil {
		// 异步发送响应
		err = c.AsyncWrite(response, func(c gnet.Conn, err error) error {
			if err != nil {
				logging.Errorf("Failed to send response: %v", err)
			}
			return nil
		})
		if err != nil {
			logging.Errorf("Failed to queue async write: %v", err)
		}
	}

	return gnet.None
}

func (h *Handler) OnTick() (delay time.Duration, action gnet.Action) {
	// 检查超时连接
	h.broker.CheckTimeouts()
	return 5 * time.Second, gnet.None
}
