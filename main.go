package main

import (
	"context"
	"github.com/panjf2000/gnet/v2"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"busy-cloud/gnet-mqtt/broker"
	"busy-cloud/gnet-mqtt/network"
)

func main() {
	// 初始化slog日志
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// 创建Broker管理器
	brokerManager := broker.NewManager(logger)

	// 创建网络处理器（用于标准TCP和WebSocket）
	netHandler := network.NewMQTTConnectionHandler(brokerManager, logger)

	// 创建标准TCP服务器
	tcpServer := network.NewTCPServer(":1885", netHandler, logger)

	// 创建WebSocket服务器
	wsServer := network.NewWebSocketServer(":1884", netHandler, logger)

	// 创建Gnet handler
	gnetHandler := &Handler{
		broker: brokerManager,
	}

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("Starting MQTT Broker with multiple protocols...")

	// 启动标准TCP服务器（goroutine）
	go func() {
		if err := tcpServer.Start(ctx); err != nil {
			slog.Error("TCP server failed", "error", err)
		}
	}()

	// 启动WebSocket服务器（goroutine）
	go func() {
		if err := wsServer.Start(ctx); err != nil {
			slog.Error("WebSocket server failed", "error", err)
		}
	}()

	slog.Info("MQTT Broker started successfully",
		"gnet_tcp", 1883,
		"std_tcp", 1885,
		"websocket", 1884)

	// 启动Gnet服务器（主服务器，阻塞）
	err := gnet.Run(gnetHandler,
		"tcp://:1883",
		gnet.WithMulticore(true),
		gnet.WithReusePort(true),
		gnet.WithTCPNoDelay(gnet.TCPNoDelay),
	)
	if err != nil {
		slog.Error("Gnet server failed", "error", err)
		return
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	slog.Info("Shutting down MQTT Broker...")

	// 优雅关闭
	tcpServer.Stop()
	wsServer.Stop()
	cancel()

	slog.Info("MQTT Broker stopped")
}
