package network

import (
	"net"

	"busy-cloud/gnet-mqtt/types"
)

// TCPConn 包装标准net.Conn实现types.Conn
type TCPConn struct {
	conn net.Conn
}

func NewTCPConn(conn net.Conn) types.Conn {
	return &TCPConn{conn: conn}
}

func (t *TCPConn) Read(b []byte) (n int, err error) {
	return t.conn.Read(b)
}

func (t *TCPConn) Write(b []byte) (n int, err error) {
	return t.conn.Write(b)
}

func (t *TCPConn) Close() error {
	return t.conn.Close()
}

func (t *TCPConn) RemoteAddr() net.Addr {
	return t.conn.RemoteAddr()
}

// ConnHandler 连接处理器接口
type ConnHandler interface {
	OnOpen(conn types.Conn)
	OnMessage(conn types.Conn, data []byte)
	OnClose(conn types.Conn, err error)
}
