package network

import (
	"net"

	"busy-cloud/gnet-mqtt/types"
	"github.com/panjf2000/gnet/v2"
)

// GNetConn gnet连接包装器
type GNetConn struct {
	conn gnet.Conn
}

// NewGNetConn 创建Gnet连接包装器
func NewGNetConn(conn gnet.Conn) types.Conn {
	return &GNetConn{conn: conn}
}

func (g *GNetConn) Read(b []byte) (n int, err error) {
	return 0, nil // gnet在OnTraffic中处理读取
}

func (g *GNetConn) Write(b []byte) (n int, err error) {
	err = g.conn.AsyncWrite(b, nil)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (g *GNetConn) Close() error {
	return g.conn.Close()
}

func (g *GNetConn) RemoteAddr() net.Addr {
	return g.conn.RemoteAddr()
}
