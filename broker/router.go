package broker

import (
	"strings"
	"sync"

	"busy-cloud/gnet-mqtt/types"
)

// Router 主题路由器
type Router struct {
	subscriptions    map[string]map[string]byte // topic -> clientID -> QoS
	retainedMessages map[string]*types.Message  // topic -> message
	mu               sync.RWMutex
}

// NewRouter 创建新的路由器
func NewRouter() *Router {
	return &Router{
		subscriptions:    make(map[string]map[string]byte),
		retainedMessages: make(map[string]*types.Message),
	}
}

// Subscribe 添加订阅
func (r *Router) Subscribe(clientID string, topicFilter string, qos byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.subscriptions[topicFilter] == nil {
		r.subscriptions[topicFilter] = make(map[string]byte)
	}
	r.subscriptions[topicFilter][clientID] = qos
}

// Unsubscribe 取消订阅
func (r *Router) Unsubscribe(clientID string, topicFilter string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if clients, exists := r.subscriptions[topicFilter]; exists {
		delete(clients, clientID)
		if len(clients) == 0 {
			delete(r.subscriptions, topicFilter)
		}
	}
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
}

// RouteMessage 路由消息
func (r *Router) RouteMessage(message *types.Message) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 处理保留消息
	if message.Retain {
		if len(message.Payload) == 0 {
			// 空载荷表示删除保留消息
			delete(r.retainedMessages, message.Topic)
		} else {
			// 设置保留消息
			r.retainedMessages[message.Topic] = message
		}
	}

	// 查找匹配的订阅者
	matchedClients := make(map[string]byte)

	for topicFilter, clients := range r.subscriptions {
		if r.matchTopic(message.Topic, topicFilter) {
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

	// 这里应该将消息发送给匹配的客户端
	// 在实际实现中，这里会调用发送逻辑
	r.deliverToClients(message, matchedClients)
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

// deliverToClients 投递消息给客户端
func (r *Router) deliverToClients(message *types.Message, clients map[string]byte) {
	// 这里应该实现实际的消息发送逻辑
	// 目前只是打印日志
	for clientID, qos := range clients {
		_ = clientID
		_ = qos
		// TODO: 实际发送消息给客户端
		// 需要访问Manager中的clients映射来获取连接
	}
}

// GetRetainedMessage 获取保留消息
func (r *Router) GetRetainedMessage(topic string) *types.Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.retainedMessages[topic]
}
