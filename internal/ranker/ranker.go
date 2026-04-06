package ranker

import (
	"math/rand"
	"validation-sim/internal/agent"
)

// Mô hình Ranker mô phỏng sự hạn chế phạm vi tiếp cận khác biệt của thuật toán nền tảng

// / Thông tin nghiên cứu từ đề xuất:
// - Bài đăng chuyên sâu/đầu tư nhiều công sức -> phạm vi tiếp cận hạn chế (ít lượt thích, người đăng cảm thấy bị bỏ qua)
// - Bài đăng ít đầu tư/theo xu hướng -> phạm vi tiếp cận được tăng cường (lượt thích tăng đột biến, thông báo tràn ngập)
//
// Điều này tạo ra vòng lặp tự xác thực không hiệu quả theo thời gian
type Ranker struct {
	// SuppressionCoefficient κ: mức độ mạnh mẽ của việc ngăn chặn nội dung nhạy cảm
	// Range 0.0 (không có độ lệch) to 1.0 (giảm thiểu tối đa)
	SuppressionCoefficient float64

	// EchoChamberStrength: xác suất một bài đăng chỉ được hiển thị cho các agent có cùng quan điểm
	EchoChamberStrength float64
}

// ReachResult cho hub biết có bao nhiêu agent sẽ thấy bài đăng này
// và liệu nó có nhận được hệ số nhân xác thực hay không
type ReachResult struct {
	ReachFraction        float64 // 0.0–1.0 of total agent population
	ValidationMultiplier float64 // applied to base validation points
	IsEchoChamber        bool
}

func New(kappa, echoStrength float64) *Ranker {
	return &Ranker{
		SuppressionCoefficient: kappa,
		EchoChamberStrength:    echoStrength,
	}
}

// Evaluate trả về hồ sơ phạm vi tiếp cận cho một hành động nhất định
func (r *Ranker) Evaluate(action agent.Action) ReachResult {
	switch action.ContentType {
	case agent.DeepContent:
		return r.deepContentResult()
	case agent.TrendContent:
		return r.trendContentResult()
	}
	return ReachResult{ReachFraction: 0.1, ValidationMultiplier: 1.0}
}

// deepContentResult: phạm vi tiếp cận hạn chế -> tác nhân đăng nội dung chuyên sâu, bị phớt lờ -> sự thất vọng
func (r *Ranker) deepContentResult() ReachResult {
	// Base reach severely reduced by suppression coefficient
	reach := (1.0 - r.SuppressionCoefficient) * (0.05 + rand.Float64()*0.10)
	return ReachResult{
		ReachFraction:        reach,
		ValidationMultiplier: 0.4, // even if seen, fewer likes are awarded :]
		IsEchoChamber:        false,
	}
}

// trendContentResult: phạm vi tiếp cận cao + sự bùng nổ xác thực -> mô phỏng "lan truyền mạnh"
func (r *Ranker) trendContentResult() ReachResult {
	// Trend nhận được sự gia tăng phạm vi tiếp cận tỷ lệ thuận với việc hạn chế nội dung ít người biết đến.
	reach := 0.3 + r.SuppressionCoefficient*0.5 + rand.Float64()*0.2
	if reach > 1.0 {
		reach = 1.0
	}

	// Echo chamber: xác suất cao chỉ được hiển thị cho những người đã đồng ý
	isEcho := rand.Float64() < r.EchoChamberStrength
	multiplier := 2.5
	if isEcho {
		multiplier = 4.0 // echo chamber khuếch đại sự xác thực hơn nữa
	}

	return ReachResult{
		ReachFraction:        reach,
		ValidationMultiplier: multiplier,
		IsEchoChamber:        isEcho,
	}
}
