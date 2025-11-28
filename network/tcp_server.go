package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

// TCPServer 标准Net TCP服务器
type TCPServer struct {
	address  string
	handler  ConnHandler
	listener net.Listener
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	logger   *slog.Logger
}

// NewTCPServer 创建新的TCP服务器
func NewTCPServer(address string, handler ConnHandler, logger *slog.Logger) *TCPServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &TCPServer{
		address: address,
		handler: handler,
		logger:  logger,
	}
}

// Start 启动TCP服务器
func (s *TCPServer) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	var err error
	s.listener, err = net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.address, err)
	}

	s.logger.Info("TCP server started", "address", s.address)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop 停止TCP服务器
func (s *TCPServer) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	s.logger.Info("TCP server stopped")
	return nil
}

// acceptLoop 接受连接循环
func (s *TCPServer) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.logger.Error("Failed to accept connection", "error", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection 处理单个连接
func (s *TCPServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	s.logger.Debug("New TCP connection", "remote_addr", conn.RemoteAddr().String())

	// 包装为标准连接
	tcpConn := NewTCPConn(conn)

	// 通知处理器有新连接
	s.handler.OnOpen(tcpConn)

	// 读取循环
	buffer := make([]byte, 4096)
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			n, err := conn.Read(buffer)
			if err != nil {
				s.handler.OnClose(tcpConn, err)
				return
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buffer[:n])
				s.handler.OnMessage(tcpConn, data)
			}
		}
	}
}
