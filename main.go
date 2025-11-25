package main

import (
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

func main() {
	// 创建handler实例
	handler := &Handler{}

	// 启动MQTT Broker
	err := gnet.Run(handler,
		"tcp://:1883",
		gnet.WithMulticore(true),
		gnet.WithReusePort(true),
		gnet.WithLogLevel(logging.InfoLevel),
		gnet.WithTCPNoDelay(gnet.TCPNoDelay),
	)

	if err != nil {
		logging.Fatalf("Failed to start MQTT Broker: %v", err)
	}
}
