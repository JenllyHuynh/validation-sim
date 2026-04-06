package hub

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"validation-sim/internal/agent"
	"validation-sim/internal/ranker"
	"validation-sim/internal/validation"
)

// Hub là bộ điều phối trung tâm
// Nó nhận tất cả các hành động từ các tác nhân, xử lý chúng thông qua bộ xếp hạng,
// sau đó hỏi công cụ xác thực xem có nên thưởng hay không - và định tuyến các sự kiện trở lại
// Architecture:một goroutine của hub xử lý kênh hành động dùng chung
// ngăn không cho bất kỳ tác nhân nào bị tắc nghẽn khi hộp thư đi đầy
type Hub struct {
	mu sync.RWMutex

	agents          map[string]*agent.Agent
	validationChans map[string]chan agent.ValidationEvent // hub -> each agent
	engine          *validation.Engine
	ranker          *ranker.Ranker
	actionIn        chan agent.Action

	// Leaderboard: agentID -> validationScore (updated after every reward)
	leaderboard map[string]float64

	// Bộ tích lũy số liệu cho bộ thu thập số liệu
	RewardLog   []RewardRecord
	RewardLogMu sync.Mutex
}

type RewardRecord struct {
	AgentID     string
	Points      float64
	IsBurst     bool
	Timestamp   time.Time
	ContentType agent.ContentType
}

func New(eng *validation.Engine, rnk *ranker.Ranker, bufSize int) *Hub {
	return &Hub{
		agents:          make(map[string]*agent.Agent),
		validationChans: make(map[string]chan agent.ValidationEvent),
		engine:          eng,
		ranker:          rnk,
		actionIn:        make(chan agent.Action, bufSize),
		leaderboard:     make(map[string]float64),

		// Khởi tạo RewardLog và mutex riêng
		RewardLog:   make([]RewardRecord, 0, 2000),
		RewardLogMu: sync.Mutex{},
	}
}

// RegisterAgent kết nối một tác nhân vào trung tâm và trả về kênh xác thực của nó
func (h *Hub) RegisterAgent(a *agent.Agent) chan agent.ValidationEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	validCh := make(chan agent.ValidationEvent, 32)
	h.agents[a.ID] = a
	h.validationChans[a.ID] = validCh
	h.leaderboard[a.ID] = 0
	return validCh
}

// ActionChannel trả về kênh đến dùng chung cho tất cả các nhân viên để ghi tin nhắn
func (h *Hub) ActionChannel() chan<- agent.Action {
	return h.actionIn
}

// Run khởi động vòng lặp xử lý của hub - gọi như một goroutine
func (h *Hub) Run(quit <-chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			return

		case action := <-h.actionIn:
			h.processAction(action)

		case <-ticker.C:
			// Thông báo bảng xếp hạng định kỳ - gửi thông báo so sánh trên mạng xã hội
			h.broadcastLeaderboardStatus()
		}
	}
}

func (h *Hub) processAction(action agent.Action) {
	h.mu.RLock()
	a, ok := h.agents[action.AgentID]
	validCh, chOk := h.validationChans[action.AgentID]
	h.mu.RUnlock()

	if !ok || !chOk || a == nil {
		return
	}

	// 1. CRun through content ranker - xác định phạm vi tiếp cận và hệ số nhân
	reach := h.ranker.Evaluate(action)

	// 2. Ask validation engine whether to reward (respects A/B scenario)
	decision := h.engine.Evaluate(a, action)

	if !decision.ShouldReward {
		return
	}

	// 3. Apply ranker's multiplier to the validation points
	finalPoints := decision.Points * reach.ValidationMultiplier

	// 4. Send reward back to agent (non-blocking - agent may be disengaged)
	event := agent.ValidationEvent{
		AgentID:        action.AgentID,
		Points:         finalPoints,
		IsNotification: false,
		Timestamp:      time.Now(),
	}

	// Non-blocking send
	select {
	case validCh <- event:
	default:
	}

	// 5. Update leaderboard + RewardLog
	h.mu.Lock()
	h.leaderboard[action.AgentID] += finalPoints
	h.RewardLog = append(h.RewardLog, RewardRecord{
		AgentID:     action.AgentID,
		Points:      finalPoints,
		IsBurst:     decision.IsBurst,
		Timestamp:   time.Now(),
		ContentType: action.ContentType,
	})
	h.mu.Unlock()
}

// broadcastLeaderboardStatus sends "top 10%" notifications to qualifying agents.
// Mô hình này thể hiện cơ chế báo hiệu địa vị - cơ chế so sánh xã hội từ RQ2.
func (h *Hub) broadcastLeaderboardStatus() {
	h.mu.RLock()
	total := len(h.leaderboard)
	if total == 0 {
		h.mu.RUnlock()
		return
	}

	// Create a safe copy of scores
	type scoreEntry struct {
		id    string
		score float64
	}
	scores := make([]scoreEntry, 0, total)
	for id, score := range h.leaderboard {
		scores = append(scores, scoreEntry{id, score})
	}
	h.mu.RUnlock()

	if total == 0 {
		return
	}

	// Sort outside of lock
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Send notifications
	for rank, s := range scores {
		if !h.engine.ShouldSendSocialComparison(rank+1, total) {
			break
		}

		h.mu.RLock()
		ch, ok := h.validationChans[s.id]
		h.mu.RUnlock()

		if !ok {
			continue
		}

		// Gửi không chặn với thời gian chờ để tránh tắc nghẽn
		select {
		case ch <- agent.ValidationEvent{
			AgentID:        s.id,
			Points:         15,
			IsNotification: true,
			Timestamp:      time.Now(),
		}:
		default:
		}
	}
}

// RegisterAgentFmt creates and registers an agent
func (h *Hub) RegisterAgentFmt(id string, p agent.PersonalityType) *agent.Agent {
	actionCh := h.ActionChannel()
	// Create validation channel first
	validCh := make(chan agent.ValidationEvent, 32)

	// Create agent with the channel
	a := agent.NewAgent(fmt.Sprintf("%s-%s", p, id), p, actionCh, validCh)

	// Register in hub
	h.mu.Lock()
	h.agents[a.ID] = a
	h.validationChans[a.ID] = validCh
	h.leaderboard[a.ID] = 0
	h.mu.Unlock()

	return a
}

func (h *Hub) AgentByID(id string) (*agent.Agent, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	a, ok := h.agents[id]
	return a, ok
}

func (h *Hub) AllAgentIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.agents))
	for id := range h.agents {
		ids = append(ids, id)
	}
	return ids
}
