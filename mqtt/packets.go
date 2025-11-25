package mqtt

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// 控制报文类型
const (
	CONNECT     = 1
	CONNACK     = 2
	PUBLISH     = 3
	PUBACK      = 4
	SUBSCRIBE   = 8
	SUBACK      = 9
	UNSUBSCRIBE = 10
	UNSUBACK    = 11
	PINGREQ     = 12
	PINGRESP    = 13
	DISCONNECT  = 14
)

// 错误定义
var (
	ErrMalformedPacket = errors.New("malformed packet")
	ErrInvalidLength   = errors.New("invalid length")
)

// ConnectPacket 连接报文
type ConnectPacket struct {
	ProtocolName  string
	ProtocolLevel byte
	CleanSession  bool
	WillFlag      bool
	WillQoS       byte
	WillRetain    bool
	UsernameFlag  bool
	PasswordFlag  bool
	KeepAlive     uint16
	ClientID      string
	WillTopic     string
	WillMessage   []byte
	Username      string
	Password      []byte
}

// PublishPacket 发布报文
type PublishPacket struct {
	TopicName string
	Payload   []byte
	QoS       byte
	PacketID  uint16
	Retain    bool
	Dup       bool
}

// SubscribePacket 订阅报文
type SubscribePacket struct {
	PacketID uint16
	Topics   []SubscribeTopic
}

type SubscribeTopic struct {
	TopicFilter string
	QoS         byte
}

// PingReqPacket 心跳请求
type PingReqPacket struct{}

// DisconnectPacket 断开连接
type DisconnectPacket struct{}

// packetReader 辅助读取器
type packetReader struct {
	buf []byte
	pos int
}

func (r *packetReader) readByte() byte {
	b := r.buf[r.pos]
	r.pos++
	return b
}

func (r *packetReader) readBytes(n int) []byte {
	data := r.buf[r.pos : r.pos+n]
	r.pos += n
	return data
}

func (r *packetReader) remaining() int {
	return len(r.buf) - r.pos
}

// readString 读取UTF-8字符串
func readString(r *packetReader) (string, error) {
	if r.remaining() < 2 {
		return "", ErrMalformedPacket
	}

	lengthBuf := r.readBytes(2)
	length := binary.BigEndian.Uint16(lengthBuf)

	if length == 0 {
		return "", nil
	}

	if r.remaining() < int(length) {
		return "", ErrMalformedPacket
	}

	strBuf := r.readBytes(int(length))
	return string(strBuf), nil
}

// readLength 读取可变字节整数
func readLength(r *packetReader) (int, error) {
	var multiplier = 1
	var value = 0

	for {
		if r.remaining() < 1 {
			return 0, ErrMalformedPacket
		}

		encodedByte := r.readByte()
		value += int(encodedByte&127) * multiplier

		if multiplier > 128*128*128 {
			return 0, ErrInvalidLength
		}
		multiplier *= 128

		if (encodedByte & 128) == 0 {
			break
		}
	}

	return value, nil
}

// DecodePacket 解析MQTT报文
func DecodePacket(data []byte) (interface{}, error) {
	if len(data) < 2 {
		return nil, ErrMalformedPacket
	}

	reader := &packetReader{buf: data}

	// 读取固定头
	fixedHeader := reader.readByte()
	packetType := fixedHeader >> 4
	flags := fixedHeader & 0x0F

	// 读取剩余长度
	remainingLength, err := readLength(reader)
	if err != nil {
		return nil, err
	}

	// 检查数据长度是否足够
	if reader.remaining() < remainingLength {
		return nil, ErrMalformedPacket
	}

	// 根据报文类型解析
	switch packetType {
	case CONNECT:
		return decodeConnectPacket(reader, flags)
	case PUBLISH:
		return decodePublishPacket(reader, flags, remainingLength)
	case SUBSCRIBE:
		return decodeSubscribePacket(reader, flags)
	case PINGREQ:
		return &PingReqPacket{}, nil
	case DISCONNECT:
		return &DisconnectPacket{}, nil
	default:
		return nil, fmt.Errorf("unsupported packet type: %d", packetType)
	}
}

// decodeConnectPacket 解析CONNECT报文
func decodeConnectPacket(r *packetReader, flags byte) (*ConnectPacket, error) {
	p := &ConnectPacket{}

	var err error
	p.ProtocolName, err = readString(r)
	if err != nil {
		return nil, err
	}

	if r.remaining() < 1 {
		return nil, ErrMalformedPacket
	}
	p.ProtocolLevel = r.readByte()

	if r.remaining() < 1 {
		return nil, ErrMalformedPacket
	}
	connectFlags := r.readByte()

	p.CleanSession = (connectFlags & 0x02) != 0
	p.WillFlag = (connectFlags & 0x04) != 0
	p.WillQoS = (connectFlags >> 3) & 0x03
	p.WillRetain = (connectFlags & 0x20) != 0
	p.PasswordFlag = (connectFlags & 0x40) != 0
	p.UsernameFlag = (connectFlags & 0x80) != 0

	if r.remaining() < 2 {
		return nil, ErrMalformedPacket
	}
	keepAliveBuf := r.readBytes(2)
	p.KeepAlive = binary.BigEndian.Uint16(keepAliveBuf)

	p.ClientID, err = readString(r)
	if err != nil {
		return nil, err
	}

	if p.WillFlag {
		p.WillTopic, err = readString(r)
		if err != nil {
			return nil, err
		}

		if r.remaining() < 2 {
			return nil, ErrMalformedPacket
		}
		willMsgLenBuf := r.readBytes(2)
		willMsgLen := binary.BigEndian.Uint16(willMsgLenBuf)

		if r.remaining() < int(willMsgLen) {
			return nil, ErrMalformedPacket
		}
		p.WillMessage = r.readBytes(int(willMsgLen))
	}

	if p.UsernameFlag {
		p.Username, err = readString(r)
		if err != nil {
			return nil, err
		}
	}

	if p.PasswordFlag {
		if r.remaining() < 2 {
			return nil, ErrMalformedPacket
		}
		pwdLenBuf := r.readBytes(2)
		pwdLen := binary.BigEndian.Uint16(pwdLenBuf)

		if r.remaining() < int(pwdLen) {
			return nil, ErrMalformedPacket
		}
		p.Password = r.readBytes(int(pwdLen))
	}

	return p, nil
}

// decodePublishPacket 解析PUBLISH报文
func decodePublishPacket(r *packetReader, flags byte, remainingLength int) (*PublishPacket, error) {
	p := &PublishPacket{}

	// 解析标志位
	p.Dup = (flags & 0x08) != 0
	p.QoS = (flags >> 1) & 0x03
	p.Retain = (flags & 0x01) != 0

	// 读取主题名
	var err error
	p.TopicName, err = readString(r)
	if err != nil {
		return nil, err
	}

	// QoS > 0 时有PacketID
	if p.QoS > 0 {
		if r.remaining() < 2 {
			return nil, ErrMalformedPacket
		}
		packetIDBuf := r.readBytes(2)
		p.PacketID = binary.BigEndian.Uint16(packetIDBuf)
	}

	// 剩余的都是有效载荷
	payloadLength := remainingLength - (r.pos - 1) // 减去已读取的部分
	if payloadLength > 0 {
		p.Payload = r.readBytes(payloadLength)
	}

	return p, nil
}

// decodeSubscribePacket 解析SUBSCRIBE报文
func decodeSubscribePacket(r *packetReader, flags byte) (*SubscribePacket, error) {
	p := &SubscribePacket{}

	// 读取PacketID
	if r.remaining() < 2 {
		return nil, ErrMalformedPacket
	}
	packetIDBuf := r.readBytes(2)
	p.PacketID = binary.BigEndian.Uint16(packetIDBuf)

	// 读取主题过滤器列表
	for r.remaining() > 0 {
		topicFilter, err := readString(r)
		if err != nil {
			return nil, err
		}

		if r.remaining() < 1 {
			return nil, ErrMalformedPacket
		}
		qos := r.readByte() & 0x03

		p.Topics = append(p.Topics, SubscribeTopic{
			TopicFilter: topicFilter,
			QoS:         qos,
		})
	}

	return p, nil
}
