package main

import (
	"github.com/panjf2000/gnet/v2"
	"time"
)

type Handler struct {
	eng gnet.Engine
}

func (h *Handler) OnBoot(eng gnet.Engine) (action gnet.Action) {
	h.eng = eng //缓存起来
	return
}

func (h *Handler) OnShutdown(eng gnet.Engine) {

}

func (h *Handler) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	return
}

func (h *Handler) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	return
}

func (h *Handler) OnTraffic(c gnet.Conn) (action gnet.Action) {
	// 读取所有可用数据
	buf, _ := c.Next(c.InboundBuffered())
	_ = c.AsyncWrite(buf, nil)
	return
}

func (h *Handler) OnTick() (delay time.Duration, action gnet.Action) {
	return
}
