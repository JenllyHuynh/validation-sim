package metrics

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"validation-sim/internal/agent"
	"validation-sim/internal/hub"
)

// Collector  dữ liệu mô phỏng và tạo ra các kết quả thống kê
// cần thiết để kiểm tra H1 (khả năng ghi nhớ), H2 (sự thay đổi nội dung), H3 (độ nhạy cảm về tính cách)
type Collector struct {
	mu        sync.Mutex
	hub       *hub.Hub
	snapshots []Snapshot
	startTime time.Time
}

// Snapshot is one point-in-time measurement of all agents
type Snapshot struct {
	Tick         int
	Time         time.Time
	AgentMetrics []AgentMetric
}

type AgentMetric struct {
	ID              string
	Personality     agent.PersonalityType
	CurrentDopamine float64
	ValidationScore float64
	ActiveTime      time.Duration
	LowEffortRatio  float64 // CQDI
	PostCount       int
	IsDisengaged    bool
}

type Summary struct {
	Scenario                string
	TotalAgents             int
	MeanRetentionSec        float64 // Δt for H1
	RetentionByPersonality  map[string]float64
	MeanCQDI                float64 // Content Quality Drift Index for H2
	CQDIByPersonality       map[string]float64
	MeanValidationScore     float64
	ValidationByPersonality map[string]float64
	DisengagementRate       float64 // fraction that quit early
}

func New(h *hub.Hub) *Collector {
	return &Collector{hub: h, startTime: time.Now()}
}

// Collect takes a snapshot every interval
func (c *Collector) Collect(interval time.Duration, quit <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	tick := 0

	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			tick++
			c.snapshot(tick)
		}
	}
}

func (c *Collector) snapshot(tick int) {
	if c.hub == nil {
		return
	}

	ids := c.hub.AllAgentIDs()
	metrics := make([]AgentMetric, 0, len(ids))

	// Sau khi fix deadlock ở tick(), lock agent là an toàn - không cần timeout goroutine nữa
	// nested goroutine + 10ms timeout cũ là workaround cho bug deadlock, không phải fix thật
	for _, id := range ids {
		a, ok := c.hub.AgentByID(id)
		if !ok || a == nil {
			continue
		}
		a.Lock()
		m := AgentMetric{
			ID:              a.ID,
			Personality:     a.Personality,
			CurrentDopamine: a.CurrentDopamine,
			ValidationScore: a.ValidationScore,
			ActiveTime:      a.ActiveTime,
			LowEffortRatio:  a.LowEffortRatio,
			PostCount:       a.PostCount,
		}
		a.Unlock()
		m.IsDisengaged = a.IsDisengaged()
		metrics = append(metrics, m)
	}

	c.mu.Lock()
	c.snapshots = append(c.snapshots, Snapshot{
		Tick:         tick,
		Time:         time.Now(),
		AgentMetrics: metrics,
	})
	c.mu.Unlock()
}

// Summarise computes final statistics ( cảm giác có bug nhưng chưa chứng minh được )
func (c *Collector) Summarise(scenarioName string) Summary {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.snapshots) == 0 {
		return Summary{Scenario: scenarioName}
	}

	// Use final snapshot for per-agent state
	last := c.snapshots[len(c.snapshots)-1]

	retentionByP := map[string][]float64{}
	cqdiByP := map[string][]float64{}
	scoreByP := map[string][]float64{}
	disengaged := 0

	for _, m := range last.AgentMetrics {
		p := m.Personality.String()
		retSec := m.ActiveTime.Seconds()
		if retSec == 0 {
			// Still running - count total elapsed
			retSec = time.Since(c.startTime).Seconds()
		}
		retentionByP[p] = append(retentionByP[p], retSec)
		cqdiByP[p] = append(cqdiByP[p], m.LowEffortRatio)
		scoreByP[p] = append(scoreByP[p], m.ValidationScore)
		if m.IsDisengaged {
			disengaged++
		}
	}

	sum := Summary{
		Scenario:                scenarioName,
		TotalAgents:             len(last.AgentMetrics),
		RetentionByPersonality:  meanMap(retentionByP),
		CQDIByPersonality:       meanMap(cqdiByP),
		ValidationByPersonality: meanMap(scoreByP),
		DisengagementRate:       float64(disengaged) / float64(len(last.AgentMetrics)),
	}
	sum.MeanRetentionSec = grandMean(retentionByP)
	sum.MeanCQDI = grandMean(cqdiByP)
	sum.MeanValidationScore = grandMean(scoreByP)
	return sum
}

// PrintSummary
func (s Summary) Print() {
	fmt.Printf("\n SIMULATION RESULTS: %s \n", s.Scenario)
	fmt.Printf("Total agents:        %d\n", s.TotalAgents)
	fmt.Printf("Mean retention (s):  %.2f\n", s.MeanRetentionSec)
	fmt.Printf("Mean CQDI:           %.3f  (0=all deep, 1=all low-effort)\n", s.MeanCQDI)
	fmt.Printf("Mean valid. score:   %.2f\n", s.MeanValidationScore)
	fmt.Printf("Disengagement rate:  %.1f%%\n", s.DisengagementRate*100)

	fmt.Println("\nRetention by personality")
	for _, p := range sortedKeys(s.RetentionByPersonality) {
		fmt.Printf("  %-12s %.2fs\n", p, s.RetentionByPersonality[p])
	}

	fmt.Println("\nCQDI (content quality drift) by personality")
	for _, p := range sortedKeys(s.CQDIByPersonality) {
		fmt.Printf("  %-12s %.3f\n", p, s.CQDIByPersonality[p])
	}

	fmt.Println("\nValidation score by personality")
	for _, p := range sortedKeys(s.ValidationByPersonality) {
		fmt.Printf("  %-12s %.2f\n", p, s.ValidationByPersonality[p])
	}
}

func meanMap(m map[string][]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, vs := range m {
		if len(vs) == 0 {
			continue
		}
		sum := 0.0
		for _, v := range vs {
			sum += v
		}
		out[k] = sum / float64(len(vs))
	}
	return out
}

func grandMean(m map[string][]float64) float64 {
	total, count := 0.0, 0
	for _, vs := range m {
		for _, v := range vs {
			total += v
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
