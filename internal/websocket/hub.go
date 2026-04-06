package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MessageType định nghĩa loại message gửi đến client
type MessageType string

const (
	MsgFeedPost       MessageType = "FEED_POST"
	MsgDopamineUpdate MessageType = "DOPAMINE_UPDATE"
	MsgNotification   MessageType = "NOTIFICATION"
	MsgPhaseChange    MessageType = "PHASE_CHANGE"
	MsgAgentStats     MessageType = "AGENT_STATS"
	MsgSimSummary     MessageType = "SIM_SUMMARY"
	MsgLeaderboard    MessageType = "LEADERBOARD"
)

// Phase của Blind Experience
type Phase int

const (
	PhaseBlind      Phase = 1 // 0-50%: không biết gì
	PhaseCorruption Phase = 2 // 50-75%: bắt đầu "tha hóa"
	PhaseReveal     Phase = 3 // 75%+: dopamine bar xuất hiện
)

// WSMessage là struct gửi xuống client qua WebSocket
type WSMessage struct {
	Type      MessageType `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// FeedPost - một bài đăng xuất hiện trên feed
type FeedPost struct {
	PostID      string  `json:"postId"`
	AuthorID    string  `json:"authorId"`
	AuthorType  string  `json:"authorType"`  // "human" | "agent"
	ContentType string  `json:"contentType"` // "deep" | "trend"
	Content     string  `json:"content"`
	Likes       int     `json:"likes"`
	IsEcho      bool    `json:"isEcho"`
	ReachPct    float64 `json:"reachPct"`
}

// DopamineUpdate - cập nhật trạng thái dopamine của human agent
type DopamineUpdate struct {
	Current   float64 `json:"current"`
	Threshold float64 `json:"threshold"`
	MaxLevel  float64 `json:"maxLevel"`
	Phase     Phase   `json:"phase"`
	Inflation float64 `json:"inflation"` // ET inflation factor
}

// Notification - thông báo "top 10%", like burst...
type Notification struct {
	Kind    string  `json:"kind"` // "top10", "like_burst", "trend_alert"
	Message string  `json:"message"`
	Points  float64 `json:"points"`
	IsBurst bool    `json:"isBurst"`
}

// AgentStats - snapshot trạng thái tất cả agents
type AgentStats struct {
	TotalAgents     int                     `json:"totalAgents"`
	ByPersonality   map[string]PersonalStat `json:"byPersonality"`
	MeanDopamine    float64                 `json:"meanDopamine"`
	DisengagedCount int                     `json:"disengagedCount"`
	TrendPostPct    float64                 `json:"trendPostPct"`
}

type PersonalStat struct {
	Count        int     `json:"count"`
	MeanDopamine float64 `json:"meanDopamine"`
	MeanScore    float64 `json:"meanScore"`
	Disengaged   int     `json:"disengaged"`
}

// LeaderboardEntry
type LeaderboardEntry struct {
	AgentID     string  `json:"agentId"`
	Personality string  `json:"personality"`
	Score       float64 `json:"score"`
	Rank        int     `json:"rank"`
	IsHuman     bool    `json:"isHuman"`
}

// PhaseChangePayload
type PhaseChangePayload struct {
	Phase     Phase   `json:"phase"`
	ActivePct float64 `json:"activePct"` // % của sim đã trải qua
	Message   string  `json:"message"`
}

// WebSocket Hub
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // dev mode - production cần restrict origin
	},
}

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 512),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// client quá chậm — drop message
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast gửi message tới tất cả connected clients
func (h *Hub) Broadcast(msgType MessageType, payload interface{}) {
	msg := WSMessage{
		Type:      msgType,
		Payload:   payload,
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws marshal error: %v", err)
		return
	}
	select {
	case h.broadcast <- data:
	default:
		// broadcast channel full
	}
}

// ServeWS upgrades HTTP connection thành WebSocket
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	client := &Client{conn: conn, send: make(chan []byte, 256)}
	h.register <- client

	go client.writePump()
	go client.readPump(h)
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump(h *Hub) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
