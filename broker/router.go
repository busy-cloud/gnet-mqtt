package broker

import (
	"log/slog"
	"strings"
	"sync"

	"busy-cloud/gnet-mqtt/types"
)

// Router 主题路由器
type Router struct {
	subscriptions    map[string]map[string]byte // topic -> clientID -> QoS
	retainedMessages map[string]*types.Message  // topic -> message
	mu               sync.RWMutex
	logger           *slog.Logger
}

// NewRouter 创建新的路由器
func NewRouter(logger *slog.Logger) *Router {
	if logger == nil {
		logger = slog.Default()
	}
	return &Router{
		subscriptions:    make(map[string]map[string]byte),
		retainedMessages: make(map[string]*types.Message),
		logger:           logger,
	}
}

// Subscribe 添加订阅
func (r *Router) Subscribe(clientID string, topicFilter []byte, qos byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	topicKey := string(topicFilter) // 字节数组转字符串用于内部存储
	if r.subscriptions[topicKey] == nil {
		r.subscriptions[topicKey] = make(map[string]byte)
	}
	r.subscriptions[topicKey][clientID] = qos

	r.logger.Debug("Subscription added",
		"client_id", clientID,
		"topic_filter", string(topicFilter), // 日志显示时转换
		"qos", qos)
}

// Unsubscribe 取消订阅
func (r *Router) Unsubscribe(clientID string, topicFilter []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	topicKey := string(topicFilter) // 字节数组转字符串
	if clients, exists := r.subscriptions[topicKey]; exists {
		delete(clients, clientID)
		if len(clients) == 0 {
			delete(r.subscriptions, topicKey)
		}
	}

	r.logger.Debug("Subscription removed",
		"client_id", clientID,
		"topic_filter", string(topicFilter)) // 日志显示时转换
}

// UnsubscribeAll 取消客户端的所有订阅
func (r *Router) UnsubscribeAll(clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for topicFilter, clients := range r.subscriptions {
		delete(clients, clientID)
		if len(clients) == 0 {
			delete(r.subscriptions, topicFilter)
		}
	}

	r.logger.Debug("All subscriptions removed", "client_id", clientID)
}

// RouteMessage 路由消息
func (r *Router) RouteMessage(message *types.Message) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	topic := string(message.Topic) // 字节数组转字符串用于匹配

	// 处理保留消息
	if message.Retain {
		if len(message.Payload) == 0 {
			// 空载荷表示删除保留消息
			delete(r.retainedMessages, topic)
			r.logger.Debug("Retained message deleted", "topic", topic)
		} else {
			// 设置保留消息
			r.retainedMessages[topic] = message
			r.logger.Debug("Retained message set",
				"topic", topic,
				"payload_size", len(message.Payload))
		}
	}

	// 查找匹配的订阅者
	matchedClients := make(map[string]byte)

	for topicFilter, clients := range r.subscriptions {
		if r.matchTopic(topic, topicFilter) {
			for clientID, qos := range clients {
				// 使用订阅的QoS和消息QoS中较小的一个
				grantedQoS := message.QoS
				if qos < grantedQoS {
					grantedQoS = qos
				}
				matchedClients[clientID] = grantedQoS
			}
		}
	}

	r.logger.Debug("Message routed",
		"topic", topic,
		"matched_clients", len(matchedClients),
		"retain", message.Retain)

	// 这里应该调用管理器的方法来实际发送消息给客户端
	// 目前只是记录日志
	for clientID := range matchedClients {
		r.logger.Debug("Message should be delivered to client",
			"topic", topic,
			"client_id", clientID)
	}
}

// matchTopic 匹配主题和主题过滤器
func (r *Router) matchTopic(topic string, filter string) bool {
	topicParts := strings.Split(topic, "/")
	filterParts := strings.Split(filter, "/")

	for i := 0; i < len(filterParts) && i < len(topicParts); i++ {
		if filterParts[i] == "#" {
			return true
		}
		if filterParts[i] != "+" && filterParts[i] != topicParts[i] {
			return false
		}
	}

	return len(topicParts) == len(filterParts)
}

// GetRetainedMessage 获取保留消息
func (r *Router) GetRetainedMessage(topic []byte) *types.Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.retainedMessages[string(topic)] // 字节数组转字符串
}

// GetSubscriptions 获取所有订阅（用于调试）
func (r *Router) GetSubscriptions() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]string)
	for topic, clients := range r.subscriptions {
		clientList := make([]string, 0, len(clients))
		for clientID := range clients {
			clientList = append(clientList, clientID)
		}
		result[topic] = clientList
	}
	return result
}
