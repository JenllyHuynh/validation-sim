package agent

import (
	"sync"
	"time"
)

// Quyết định mức độ nhạy cảm của 1 agent
type PersonalityType int

const (
	Introvert PersonalityType = iota // Nhu cầu cơ ba thấp, sự suy giảm dopamin chậm
	Extrovert                        // Mức độ trung bình, phản hồi tích cực
	Seeker                           // Mức độ cơ bản cao, cực kỳ nhạy cảm với bảng xếp hạng
)

func (p PersonalityType) String() string {
	return [...]string{"Introvert", "Extrovert", "Seeker"}[p]
}

// Phản ánh cơ chế phản giá trị: ít nỗ lực thưởng nhiều hơn
type ContentType int

const (
	DeepContent  ContentType = iota // nỗ lực cao -> bị người xếp hạng ngăn chặn
	TrendContent                    // nỗ lực thấp được tăng cường bởi xếp hạng
)

// Hành động represents one atomic event bởi agent
type Action struct {
	AgentID     string
	TargetID    string // post or comment UUID like / shared
	Kind        string // post, like, comment
	ContentType ContentType
	Timestamp   time.Time
}

// Validation send back to agent
type ValidationEvent struct {
	AgentID        string
	Points         float64 // validation was awarded
	IsNotification bool    // "u're in top 10%" trigger
	Timestamp      time.Time
}

// Agent là một người dùng ảo - mỗi người dùng chạy như một goroutine
type Agent struct {
	mu sync.Mutex

	ID          string
	Personality PersonalityType

	// Các biến trạng thái cốt lõi
	BaseValidationLevel float64 // BVL:nhu cầu nội tại về sự công nhận (0–100)
	CurrentDopamine     float64 // CD: Mức độ hài lòng hiện tại (0–100)
	EngagementThreshold float64 // ET: nơi nhân viên đang cần xác thực khẩn cấp

	// Bộ tích lũy hành vi
	ValidationScore float64       // tổng số điểm công nhận tích lũy
	ActiveTime      time.Duration // tổng thời gian tham gia
	lastActiveTick  time.Time     // last time agent hoạt động
	PostCount       int
	LowEffortRatio  float64 // CQDI contribution: tỷ lệ bài đăng có mức nỗ lực thấp

	// Channels - agent đọc validation + writes actions
	ActionOut    chan<- Action
	ValidationIn <-chan ValidationEvent

	// Lifecycle
	StartedAt time.Time
	quit      chan struct{}
}

// DopamineDecayRate return tốc độ giảm CD/tick (ersonality-dependent)
func (a *Agent) DopamineDecayRate() float64 {
	switch a.Personality {
	case Introvert:
		return 0.3 // slow - ít phụ thuộc vào tín hiệu bên ngoài

	case Extrovert:
		return 0.7

	case Seeker:
		return 1.4 // fast decay -> khao khát sự công nhận một cách cấp thiết hơn.
	default:
		return 0.5
	}
}

// IsDisengaged return true khi agent giảm xuống dưới ngưỡng cho phép
// đủ lâu để voluntarily leave the platform
func (a *Agent) IsDisengaged() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.CurrentDopamine < (a.EngagementThreshold * 0.25)
}

// ApplyValidation updates internal state khi nhận được ValidationEvent
func (a *Agent) ApplyValidation(e ValidationEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	newDopamine := a.CurrentDopamine + e.Points
	if newDopamine > 100 {
		newDopamine = 100
	}
	a.CurrentDopamine = newDopamine
	a.ValidationScore += e.Points

	// Reset active time tracking
	a.lastActiveTick = time.Now()
}

func min(x, y float64) float64 {
	if x < y {
		return x
	}
	return y
}

// Lock/Unlock expose the internal mutex so the validation engine
// có thể read CD and ET without data races
func (a *Agent) Lock() {
	a.mu.Lock()
}
func (a *Agent) Unlock() {
	a.mu.Unlock()
}
