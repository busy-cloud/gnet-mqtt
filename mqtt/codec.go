package mqtt

import (
	"bytes"
	"encoding/binary"

	"github.com/panjf2000/gnet/v2"
)

// MQTTCodec MQTT协议编解码器
type MQTTCodec struct {
	buffer bytes.Buffer
}

// Encode 编码 - 直接发送数据
func (mc *MQTTCodec) Encode(c gnet.Conn, buf []byte) ([]byte, error) {
	return buf, nil
}

// Decode 解码 - 处理TCP粘包
func (mc *MQTTCodec) Decode(c gnet.Conn) ([]byte, error) {
	// 读取所有可用的数据到buffer
	if c.InboundBuffered() > 0 {
		buf, _ := c.Next(-1)
		mc.buffer.Write(buf)
	}

	// 如果buffer中有数据，尝试解析
	if mc.buffer.Len() > 0 {
		data := mc.buffer.Bytes()

		// 我们需要至少2个字节才能判断包长度
		if len(data) < 2 {
			return nil, nil
		}

		// 解析包长度
		totalLength, err := mc.parsePacketLength(data)
		if err != nil {
			return nil, err
		}

		if totalLength > 0 && len(data) >= totalLength {
			// 提取完整的数据包
			packet := make([]byte, totalLength)
			copy(packet, data[:totalLength])

			// 从buffer中移除已处理的数据
			mc.buffer.Next(totalLength)

			return packet, nil
		}
	}

	return nil, nil
}

// parsePacketLength 解析MQTT包总长度
func (mc *MQTTCodec) parsePacketLength(data []byte) (int, error) {
	if len(data) < 2 {
		return 0, ErrMalformedPacket
	}

	// 跳过第一个字节(固定头)
	pos := 1
	multiplier := 1
	value := 0

	for {
		if pos >= len(data) {
			return 0, ErrMalformedPacket
		}

		encodedByte := data[pos]
		value += int(encodedByte&127) * multiplier
		multiplier *= 128
		pos++

		if multiplier > 128*128*128 {
			return 0, ErrInvalidLength
		}

		if (encodedByte & 128) == 0 {
			break
		}
	}

	// 总长度 = 固定头(1字节) + 长度字段字节数 + 剩余长度
	totalLength := 1 + (pos - 1) + value
	return totalLength, nil
}

// CreatePacket 创建各种MQTT响应包
func CreatePacket(packetType byte, payload ...[]byte) []byte {
	var packet []byte

	// 固定头
	fixedHeader := packetType << 4

	// 计算剩余长度
	remainingLength := 0
	for _, p := range payload {
		remainingLength += len(p)
	}

	// 编码剩余长度
	encodedLength := encodeLength(remainingLength)

	// 构建包
	packet = append(packet, fixedHeader)
	packet = append(packet, encodedLength...)
	for _, p := range payload {
		packet = append(packet, p...)
	}

	return packet
}

// encodeLength 编码长度
func encodeLength(length int) []byte {
	var encoded []byte

	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		encoded = append(encoded, digit)
		if length == 0 {
			break
		}
	}

	return encoded
}

// CreateConnAck 创建CONNACK包
func CreateConnAck(sessionPresent bool, returnCode byte) []byte {
	var payload []byte

	// 连接确认标志
	ackFlags := byte(0)
	if sessionPresent {
		ackFlags |= 0x01
	}
	payload = append(payload, ackFlags)
	payload = append(payload, returnCode)

	return CreatePacket(CONNACK, payload)
}

// CreatePingResp 创建PINGRESP包
func CreatePingResp() []byte {
	return CreatePacket(PINGRESP, nil)
}

// CreateSubAck 创建SUBACK包
func CreateSubAck(packetID uint16, returnCodes []byte) []byte {
	var payload []byte

	// PacketID
	packetIDBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(packetIDBuf, packetID)
	payload = append(payload, packetIDBuf...)

	// Return codes
	payload = append(payload, returnCodes...)

	return CreatePacket(SUBACK, payload)
}
