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

// Encode 编码
func (mc *MQTTCodec) Encode(c gnet.Conn, buf []byte) ([]byte, error) {
	return buf, nil
}

// Decode 解码 - 处理TCP粘包
func (mc *MQTTCodec) Decode(c gnet.Conn) ([]byte, error) {
	if c.InboundBuffered() > 0 {
		buf, _ := c.Next(-1)
		mc.buffer.Write(buf)
	}

	if mc.buffer.Len() > 0 {
		data := mc.buffer.Bytes()

		if len(data) < 2 {
			return nil, nil
		}

		totalLength, err := mc.parsePacketLength(data)
		if err != nil {
			return nil, err
		}

		if totalLength > 0 && len(data) >= totalLength {
			packet := make([]byte, totalLength)
			copy(packet, data[:totalLength])
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

	totalLength := 1 + (pos - 1) + value
	return totalLength, nil
}

// CreatePacket 创建各种MQTT响应包
func CreatePacket(packetType byte, payload ...[]byte) []byte {
	var packet []byte

	fixedHeader := packetType << 4

	remainingLength := 0
	for _, p := range payload {
		remainingLength += len(p)
	}

	encodedLength := encodeLength(remainingLength)

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

	packetIDBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(packetIDBuf, packetID)
	payload = append(payload, packetIDBuf...)

	payload = append(payload, returnCodes...)

	return CreatePacket(SUBACK, payload)
}

// EncodeBinary 编码二进制数据（UTF-8字符串）
func EncodeBinary(data []byte) []byte {
	result := make([]byte, 2+len(data))
	binary.BigEndian.PutUint16(result, uint16(len(data)))
	copy(result[2:], data)
	return result
}
