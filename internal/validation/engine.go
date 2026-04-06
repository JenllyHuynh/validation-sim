package validation

import (
	"math/rand"
	"sync"
	"time"
	"validation-sim/internal/agent"
)

// Kịch bản kiểm soát thuật toán phần thưởng nào được kích hoạt (thử nghiệm A/B)
type Scenario int

const (
	ScenarioA Scenario = iota // liner: thưởng cho mỗi N hành động
	ScenarioB                 // variable-ratio: giữ lại cho đến khi nồng độ dopamine giảm, sau đó bùng phát
)

// Hệ thống quyết định khi nào và mức độ xác thực sẽ trao cho mỗi tác nhân

// Đây là thành phần quan trọng nhất đối với nghiên cứu:
// Kịch bản B triển khai cơ chế "gacha :]" được quan sát thấy trên các nền tảng xã hội.
type Engine struct {
	mu       sync.Mutex
	scenario Scenario

	// Theo dõi từng tác nhân (agentID -> trạng thái)
	actionCounts  map[string]int
	lastDopamine  map[string]float64
	pendingReward map[string]float64 // điểm được giữ lại chờ thời điểm thích hợp

	lastRewarded map[string]time.Time
}

// ValidationDecision là giá trị mà công cụ trả về cho một tác nhân + hành động nhất định
type ValidationDecision struct {
	ShouldReward bool
	Points       float64
	IsBurst      bool // true khi điểm bị giữ lại được loại bỏ cùng một lúc
}

func NewEngine(s Scenario) *Engine {
	return &Engine{
		scenario:      s,
		actionCounts:  make(map[string]int),
		lastDopamine:  make(map[string]float64),
		pendingReward: make(map[string]float64),
		lastRewarded:  make(map[string]time.Time),
	}
}

// Hàm Evaluate quyết định xem có cấp phép xác thực cho hành động này hay không.

// Kịch bản A: Xác định - Thưởng cho mỗi hành động thứ 5
// Kịch bản B: Chiến lược
// - Tích lũy điểm đang chờ xử lý một cách âm thầm trong khi tác nhân "vui vẻ" (CD cao)
// - Giải phóng tất cả khi tác nhân thể hiện tín hiệu nhàm chán (CD gần ET)
// - Thỉnh thoảng gửi thông báo so sánh xã hội "top 10%"
func (e *Engine) Evaluate(a *agent.Agent, action agent.Action) ValidationDecision {
	if a == nil {
		return ValidationDecision{}
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	// Initialize maps if nil
	if e.actionCounts == nil {
		e.actionCounts = make(map[string]int)
	}
	if e.lastDopamine == nil {
		e.lastDopamine = make(map[string]float64)
	}
	if e.pendingReward == nil {
		e.pendingReward = make(map[string]float64)
	}
	if e.lastRewarded == nil {
		e.lastRewarded = make(map[string]time.Time)
	}
	e.actionCounts[a.ID]++
	count := e.actionCounts[a.ID]

	// Base points - nội dung chuyên sâu mang lại nhiều điểm hơn một cách tự nhiên
	// nhưng thuật toán thường hạn chế việc phân phối nội dung (see ContentRanker)
	basePoints := 5.0
	if action.ContentType == agent.DeepContent {
		basePoints = 8.0
	}
	switch e.scenario {
	case ScenarioA:
		return e.linearReward(a.ID, count, basePoints)
	case ScenarioB:
		return e.variableRatioReward(a, count, basePoints)
	}
	return ValidationDecision{}
}

// linearReward: simple N-interval schedule (control condition)
func (e *Engine) linearReward(id string, count int, pts float64) ValidationDecision {
	if count%5 == 0 {
		return ValidationDecision{ShouldReward: true, Points: pts}
	}
	return ValidationDecision{}
}

// variableRatioReward: lịch trình được tối ưu hóa bằng thuật toán

// Ba yếu tố kích hoạt:
// 1. Sự sụt giảm dopamine - giải phóng điểm tích lũy khi CD gần đạt ET
// 2. Sự bùng nổ ngẫu nhiên - phần thưởng bất ngờ thỉnh thoảng để duy trì tính khó đoán
// 3. Mức sàn tối thiểu - đảm bảo ít nhất một phần thưởng cho mỗi 20 hành động để ngăn chặn tình trạng bỏ cuộc hoàn toàn
func (e *Engine) variableRatioReward(a *agent.Agent, count int, pts float64) ValidationDecision {
	a.Lock()
	cd := a.CurrentDopamine
	et := a.EngagementThreshold
	a.Unlock()

	// Luôn tích lũy một cách âm thầm
	e.pendingReward[a.ID] += pts

	// 1. Sự sụt giảm dopamine
	// Agent đang thể hiện sự "boredom" - đây là thời điểm để tạo ra tác động tối đa
	if cd < et*1.3 && e.pendingReward[a.ID] > 0 {
		points := e.pendingReward[a.ID]
		e.pendingReward[a.ID] = 0
		e.lastRewarded[a.ID] = time.Now()
		return ValidationDecision{ShouldReward: true, Points: points, IsBurst: true}
	}

	// 2. Random surprise (15% chance)
	// Máy đánh bạc :] - unpredictability is addictive
	if rand.Float64() < 0.15 {
		points := e.pendingReward[a.ID] * 0.3
		e.pendingReward[a.ID] *= 0.7
		if points > 0 {
			return ValidationDecision{ShouldReward: true, Points: points}
		}
	}

	// 3. Mức sàn tối thiểu( ngăn ngừa việc rời bỏ hệ thống hoàn toàn )
	if count%20 == 0 && e.pendingReward[a.ID] > 0 {
		points := e.pendingReward[a.ID] * 0.5
		e.pendingReward[a.ID] *= 0.5
		return ValidationDecision{ShouldReward: true, Points: points}
	}

	return ValidationDecision{}
}

// Hàm ShouldSendSocialComparison trả về true
// khi agent đủ điều kiện nhận thông báo "bạn nằm trong top 10%" - mô hình Tín hiệu trạng thái (RQ2)
func (e *Engine) ShouldSendSocialComparison(agentRank int, totalAgents int) bool {
	threshold := int(float64(totalAgents) * 0.10)
	return agentRank <= threshold
}
