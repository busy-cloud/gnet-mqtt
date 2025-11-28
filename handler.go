package main

import (
	"time"

	"busy-cloud/gnet-mqtt/broker"
	"busy-cloud/gnet-mqtt/mqtt"
	"busy-cloud/gnet-mqtt/network"
	"github.com/panjf2000/gnet/v2"
)

type Handler struct {
	eng    gnet.Engine
	broker *broker.Manager
	codec  mqtt.MQTTCodec
}

func (h *Handler) OnBoot(eng gnet.Engine) (action gnet.Action) {
	h.eng = eng
	return gnet.None
}

func (h *Handler) OnShutdown(eng gnet.Engine) {
	// 清理资源
}

func (h *Handler) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	// 创建Gnet连接包装器并添加到broker
	gnetConn := network.NewGNetConn(c)
	h.broker.AddClient(gnetConn, "gnet")
	return nil, gnet.None
}

func (h *Handler) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	gnetConn := network.NewGNetConn(c)
	h.broker.RemoveClient(gnetConn)
	return gnet.None
}

func (h *Handler) OnTraffic(c gnet.Conn) (action gnet.Action) {
	// 使用编解码器处理数据
	packetData, err := h.codec.Decode(c)
	if err != nil {
		return gnet.Close
	}

	if packetData == nil {
		return gnet.None
	}

	// 解析MQTT报文
	packet, err := mqtt.DecodePacket(packetData)
	if err != nil {
		return gnet.Close
	}

	// 处理报文
	gnetConn := network.NewGNetConn(c)
	h.broker.HandlePacket(gnetConn, packet)

	return gnet.None
}

func (h *Handler) OnTick() (delay time.Duration, action gnet.Action) {
	h.broker.CheckTimeouts()
	return 5 * time.Second, gnet.None
}
