package websocket

import (
	"log"
	"sync"
	"time"
)

// Hub WebSocket 허브 - 모든 클라이언트 연결과 메시지 브로드캐스트를 관리
type Hub struct {
	// 등록된 클라이언트들
	clients map[*Client]bool

	// 채팅방별 클라이언트 그룹
	rooms map[string]map[*Client]bool

	// 클라이언트 등록 채널
	register chan *Client

	// 클라이언트 해제 채널
	unregister chan *Client

	// 모든 클라이언트에게 브로드캐스트할 메시지
	broadcast chan []byte

	// 특정 방에 브로드캐스트할 메시지
	roomBroadcast chan *RoomMessage

	// 동시성 제어를 위한 뮤텍스
	mutex sync.RWMutex

	// Heartbeat 간격 (기본: 30초)
	heartbeatInterval time.Duration

	// Heartbeat 타이머
	heartbeatTimer *time.Ticker

	// 허브 종료 신호
	done chan struct{}
}

// RoomMessage 특정 방으로 보낼 메시지
type RoomMessage struct {
	Room    string `json:"room"`
	Message []byte `json:"message"`
}

// ClientStats 클라이언트 통계
type ClientStats struct {
	TotalClients int               `json:"total_clients"`
	RoomClients  map[string]int    `json:"room_clients"`
	ClientsInfo  []*ClientInfo     `json:"clients_info"`
}

// ClientInfo 클라이언트 정보
type ClientInfo struct {
	ID          string    `json:"id"`
	Room        string    `json:"room"`
	UserID      *uint     `json:"user_id,omitempty"`
	SessionID   *string   `json:"session_id,omitempty"`
	ConnectedAt time.Time `json:"connected_at"`
	LastPing    time.Time `json:"last_ping"`
}

// NewHub Hub 인스턴스 생성
func NewHub() *Hub {
	return &Hub{
		clients:           make(map[*Client]bool),
		rooms:             make(map[string]map[*Client]bool),
		register:          make(chan *Client),
		unregister:        make(chan *Client),
		broadcast:         make(chan []byte),
		roomBroadcast:     make(chan *RoomMessage),
		heartbeatInterval: 30 * time.Second,
		done:              make(chan struct{}),
	}
}

// Run Hub 실행 - 고루틴에서 실행되어야 함
func (h *Hub) Run() {
	log.Println("WebSocket Hub starting...")
	
	// Heartbeat 타이머 시작
	h.heartbeatTimer = time.NewTicker(h.heartbeatInterval)
	defer h.heartbeatTimer.Stop()

	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastToAll(message)

		case roomMsg := <-h.roomBroadcast:
			h.broadcastToRoom(roomMsg.Room, roomMsg.Message)

		case <-h.heartbeatTimer.C:
			h.sendHeartbeat()

		case <-h.done:
			log.Println("WebSocket Hub shutting down...")
			return
		}
	}
}

// registerClient 클라이언트 등록
func (h *Hub) registerClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// 전체 클라이언트 목록에 추가
	h.clients[client] = true

	// 방별 클라이언트 목록에 추가
	if client.Room != "" {
		if h.rooms[client.Room] == nil {
			h.rooms[client.Room] = make(map[*Client]bool)
		}
		h.rooms[client.Room][client] = true
	}

	// 방 입장 확인 메시지 전송
	if client.Room != "" {
		roomJoinedMsg := &Message{
			Type: MessageTypeRoomJoined,
			Room: client.Room,
			Data: map[string]interface{}{
				"user_count": len(h.rooms[client.Room]),
			},
			Timestamp: time.Now(),
		}

		if err := client.SendMessage(roomJoinedMsg); err != nil {
			log.Printf("Failed to send room joined message: %v", err)
		}

		log.Printf("Client %s joined room %s (total in room: %d)", 
			client.ID, client.Room, len(h.rooms[client.Room]))
	}

	log.Printf("Client registered: %s (total: %d)", client.ID, len(h.clients))
}

// unregisterClient 클라이언트 해제
func (h *Hub) unregisterClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if _, ok := h.clients[client]; ok {
		// 전체 클라이언트 목록에서 제거
		delete(h.clients, client)

		// 방별 클라이언트 목록에서 제거
		if client.Room != "" && h.rooms[client.Room] != nil {
			delete(h.rooms[client.Room], client)
			
			// 방이 비었으면 방 제거
			if len(h.rooms[client.Room]) == 0 {
				delete(h.rooms, client.Room)
			}
		}

		// 클라이언트 연결 종료
		close(client.send)

		log.Printf("Client unregistered: %s (total: %d)", client.ID, len(h.clients))
	}
}

// broadcastToAll 모든 클라이언트에게 브로드캐스트
func (h *Hub) broadcastToAll(message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			// 전송 실패 시 클라이언트 연결 해제
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}
}

// broadcastToRoom 특정 방의 모든 클라이언트에게 브로드캐스트
func (h *Hub) broadcastToRoom(room string, message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if roomClients, exists := h.rooms[room]; exists {
		for client := range roomClients {
			select {
			case client.send <- message:
			default:
				// 전송 실패 시 클라이언트 연결 해제
				go func(c *Client) {
					h.unregister <- c
				}(client)
			}
		}
	}
}

// sendHeartbeat 모든 클라이언트에게 heartbeat 전송
func (h *Hub) sendHeartbeat() {
	heartbeatMsg := &Message{
		Type:      MessageTypePing,
		Timestamp: time.Now(),
	}

	messageBytes, err := heartbeatMsg.ToJSON()
	if err != nil {
		log.Printf("Failed to marshal heartbeat message: %v", err)
		return
	}

	h.broadcastToAll(messageBytes)
	
	// 응답하지 않는 클라이언트 정리
	h.cleanupInactiveClients()
}

// cleanupInactiveClients 비활성 클라이언트 정리
func (h *Hub) cleanupInactiveClients() {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	now := time.Now()
	inactiveTimeout := 90 * time.Second // 90초 무응답시 연결 해제

	for client := range h.clients {
		if now.Sub(client.LastPing) > inactiveTimeout {
			log.Printf("Removing inactive client: %s", client.ID)
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}
}

// GetStats 현재 허브 통계 정보 반환
func (h *Hub) GetStats() *ClientStats {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	roomClients := make(map[string]int)
	var clientsInfo []*ClientInfo

	// 방별 클라이언트 수 계산
	for room, clients := range h.rooms {
		roomClients[room] = len(clients)
	}

	// 클라이언트 정보 수집
	for client := range h.clients {
		clientsInfo = append(clientsInfo, &ClientInfo{
			ID:          client.ID,
			Room:        client.Room,
			UserID:      client.UserID,
			SessionID:   client.SessionID,
			ConnectedAt: client.ConnectedAt,
			LastPing:    client.LastPing,
		})
	}

	return &ClientStats{
		TotalClients: len(h.clients),
		RoomClients:  roomClients,
		ClientsInfo:  clientsInfo,
	}
}

// BroadcastToRoom 외부에서 특정 방에 메시지 브로드캐스트
func (h *Hub) BroadcastToRoom(room string, message []byte) {
	select {
	case h.roomBroadcast <- &RoomMessage{Room: room, Message: message}:
	default:
		log.Println("Room broadcast channel is full")
	}
}

// BroadcastToAll 외부에서 모든 클라이언트에게 메시지 브로드캐스트
func (h *Hub) BroadcastToAll(message []byte) {
	select {
	case h.broadcast <- message:
	default:
		log.Println("Broadcast channel is full")
	}
}

// RegisterClient 외부에서 클라이언트 등록
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient 외부에서 클라이언트 해제
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// Stop Hub 정지
func (h *Hub) Stop() {
	close(h.done)
}

// GetRoomClientCount 특정 방의 클라이언트 수 반환
func (h *Hub) GetRoomClientCount(room string) int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if roomClients, exists := h.rooms[room]; exists {
		return len(roomClients)
	}
	return 0
}

// 전역 Hub 인스턴스
var GlobalHub *Hub

// InitializeHub 전역 Hub 초기화
func InitializeHub() {
	GlobalHub = NewHub()
	go GlobalHub.Run()
	log.Println("Global WebSocket Hub initialized")
}