package agent

import (
	"fmt"
	"math/rand"
	"time"
)

// 1 tick = 1 đơn vị thời gian mô phỏng
const tickInterval = 50 * time.Millisecond

// Mỗi chu kỳ, tác nhân thực hiện:
// 1. Giảm lượng Dopamine hiện tại (mô phỏng sự nhàm chán)
// 2. Quyết định có đăng bài hay không (và loại bài đăng nào)
// 3. Xử lý mọi sự kiện ValidationEvents đang chờ xử lý
// 4. Kiểm tra điều kiện ngừng hoạt động
func (a *Agent) Run() {
	a.StartedAt = time.Now()
	a.lastActiveTick = time.Now()
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.quit:
			// Upload final active trước khi thoát
			a.mu.Lock()
			a.ActiveTime += time.Since(a.lastActiveTick)
			a.mu.Unlock()
			return

		case <-ticker.C:
			// Check if agent is still engaged
			if a.IsDisengaged() {
				a.mu.Lock()
				a.ActiveTime += time.Since(a.lastActiveTick)
				a.mu.Unlock()
				return
			}

			a.tick()

			// Update action time khi tick
			a.mu.Lock()
			a.ActiveTime += tickInterval
			a.lastActiveTick = time.Now()
			a.mu.Unlock()

		case e, ok := <-a.ValidationIn:
			if !ok {
				return // closed channel lại
			}
			a.ApplyValidation(e)
		}
	}
}

// tick advances one simulation step
func (a *Agent) tick() {
	// Lock chỉ để đọc/ghi state - unlock TRƯỚC khi gọi emitPost
	// Bug cũ tồn tại chỗ này: lock ở đây rồi emitPost cũng lock -> deadlock ( vấn đề kỹ năng )
	a.mu.Lock()

	// Lock chỉ để đọc/ghi state - unlock TRƯỚC khi gọi emitPost
	// Bug cũ tồn tại chỗ này: lock ở đây rồi emitPost cũng lock -> deadlock ( vấn đề kỹ năng )
	a.CurrentDopamine = max(0, a.CurrentDopamine-a.DopamineDecayRate())

	// 2. Decide whether to post this tick
	// Cảm giác khó chịu tăng lên khi dopamine giảm xuống ngưỡng máu tử
	urgency := 1.0

	if a.CurrentDopamine < a.EngagementThreshold {
		urgency = 2.5 // agent post càng trở nên điên cuồng hơn khi bị đói
	}
	shouldPost := rand.Float64() < (0.15 * urgency)
	cd := a.CurrentDopamine
	a.mu.Unlock() // unlock trước khi emitPost

	if shouldPost {
		a.emitPost(cd)
	}
}

// emitPost quyết định loại nội dung và gửi đến Action hub
// Phát hiện quan trọng từ nghiên cứu: khi lượng dopamine giảm, các tác nhân ngày càng ưa chuộng
// nội dung ít tốn công sức/theo xu hướng vì kinh nghiệm trước đây cho thấy nó được đền đáp
func (a *Agent) emitPost(currentDopamine float64) {
	// Xác suất đăng nội dung kém chất lượng tăng lên khi nồng độ dopamine giảm
	// Ở CD=100 -> 20% khả năng; ở CD=0 -> 80% khả năng

	lowEffortProb := 0.2 + 0.6*(1-(currentDopamine/100))
	ct := DeepContent
	if rand.Float64() < lowEffortProb {
		ct = TrendContent
	}

	a.mu.Lock()
	a.PostCount++
	if ct == TrendContent {
		// Update tỷ lệ nỗ lực thấp (CQDI tracker)
		total := float64(a.PostCount)
		a.LowEffortRatio = (a.LowEffortRatio*(total-1) + 1) / total
	} else {
		total := float64(a.PostCount)
		a.LowEffortRatio = (a.LowEffortRatio * (total - 1)) / total
	}
	a.mu.Unlock()

	// Non-blocking send
	select {
	case a.ActionOut <- Action{
		AgentID:     a.ID,
		TargetID:    fmt.Sprintf("post-%s-%d", a.ID, a.PostCount),
		Kind:        "post",
		ContentType: ct,
		Timestamp:   time.Now(),
	}:
	default:
		// Ngừng hoạt động nếu máy chủ đạt công suất tối đa - mô phỏng tình trạng tắc nghẽn mạng thực tế
	}
}

// Quit signals the agent to stop
func (a *Agent) Quit() {
	select {
	case <-a.quit:
	// đóng nó lại
	default:
		close(a.quit)
	}
}
func max(x, y float64) float64 {
	if x > y {
		return x
	}
	return y
}

// NewAgent tạo một Agent với buffered channels
// actionOut và validationIn được kết nối bởi hub
func NewAgent(id string, p PersonalityType, actionOut chan<- Action, validIn chan ValidationEvent) *Agent {
	bvl := personalityBVL(p)
	return &Agent{
		ID:                  id,
		Personality:         p,
		BaseValidationLevel: bvl,
		CurrentDopamine:     bvl, // thõa mãn vãi
		EngagementThreshold: bvl * 0.4,
		ActionOut:           actionOut,
		ValidationIn:        validIn,
		quit:                make(chan struct{}),
	}
}

func personalityBVL(p PersonalityType) float64 {
	switch p {
	case Introvert:
		return 30 + rand.Float64()*15
	case Extrovert:
		return 50 + rand.Float64()*20
	case Seeker:
		return 75 + rand.Float64()*20
	default:
		return 50
	}
}
