package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebSocketServer WebSocket服务器
type WebSocketServer struct {
	address string
	handler ConnHandler
	server  *http.Server
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *slog.Logger
}

// WebSocketConn WebSocket连接包装器
type WebSocketConn struct {
	conn *websocket.Conn
}

func (w *WebSocketConn) Read(b []byte) (n int, err error) {
	messageType, reader, err := w.conn.NextReader()
	if err != nil {
		return 0, err
	}

	if messageType != websocket.BinaryMessage {
		return 0, fmt.Errorf("unsupported message type: %d", messageType)
	}

	return reader.Read(b)
}

func (w *WebSocketConn) Write(b []byte) (n int, err error) {
	writer, err := w.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	defer writer.Close()

	n, err = writer.Write(b)
	return n, err
}

func (w *WebSocketConn) Close() error {
	return w.conn.Close()
}

func (w *WebSocketConn) RemoteAddr() net.Addr {
	return w.conn.RemoteAddr()
}

// NewWebSocketServer 创建新的WebSocket服务器
func NewWebSocketServer(address string, handler ConnHandler, logger *slog.Logger) *WebSocketServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &WebSocketServer{
		address: address,
		handler: handler,
		logger:  logger,
	}
}

// Start 启动WebSocket服务器
func (s *WebSocketServer) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWebSocket)

	s.server = &http.Server{
		Addr:    s.address,
		Handler: mux,
	}

	s.logger.Info("WebSocket server started", "address", s.address)

	s.wg.Add(1)
	go s.serve()

	return nil
}

// Stop 停止WebSocket服务器
func (s *WebSocketServer) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.server != nil {
		s.server.Shutdown(s.ctx)
	}
	s.wg.Wait()
	s.logger.Info("WebSocket server stopped")
	return nil
}

// serve 启动HTTP服务器
func (s *WebSocketServer) serve() {
	defer s.wg.Done()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Error("WebSocket server failed", "error", err)
	}
}

// handleWebSocket 处理WebSocket连接
func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade to WebSocket", "error", err)
		return
	}

	wsConn := &WebSocketConn{conn: conn}
	s.logger.Debug("New WebSocket connection", "remote_addr", conn.RemoteAddr().String())

	// 通知处理器有新连接
	s.handler.OnOpen(wsConn)

	defer func() {
		s.handler.OnClose(wsConn, nil)
		conn.Close()
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				s.handler.OnClose(wsConn, err)
				return
			}

			if messageType == websocket.BinaryMessage {
				s.handler.OnMessage(wsConn, data)
			}
		}
	}
}
