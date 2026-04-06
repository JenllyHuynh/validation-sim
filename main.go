package main

import (
	"fmt"
	"math/rand"
	"time"

	"validation-sim/internal/agent"
	"validation-sim/internal/hub"
	"validation-sim/internal/metrics"
	"validation-sim/internal/ranker"
	"validation-sim/internal/validation"
)

const (
	NumAgentsPerType    = 100 // 300 total agents (100 per personality)
	SimDuration         = 8 * time.Second
	MetricsInterval     = 200 * time.Millisecond
	ActionBufSize       = 4096
	SuppressionKappa    = 0.7 // content ranker aggression
	EchoChamberStrength = 0.65
)

func main() {
	fmt.Println("Validation Simulation — A/B Experiment")
	fmt.Printf("Agents per personality: %d  |  Duration: %s\n\n", NumAgentsPerType, SimDuration)

	summaryA := runScenario(validation.ScenarioA, "Scenario A (linear)")
	summaryB := runScenario(validation.ScenarioB, "Scenario B (variable-ratio)")

	summaryA.Print()
	summaryB.Print()

	// So sánh Delta - cốt lõi của H1
	fmt.Printf("\nH1 TEST: Retention Δ (B - A)\n")
	delta := summaryB.MeanRetentionSec - summaryA.MeanRetentionSec
	pct := 0.0
	if summaryA.MeanRetentionSec > 0 {
		pct = (delta / summaryA.MeanRetentionSec) * 100
	}
	fmt.Printf("Mean retention increase: %.2fs  (%.1f%%)\n", delta, pct)

	fmt.Printf("\nH2 TEST: Content Quality Drift\n")
	fmt.Printf("Scenario A CQDI: %.3f  |  Scenario B CQDI: %.3f\n",
		summaryA.MeanCQDI, summaryB.MeanCQDI)

	fmt.Printf("\nH3 TEST: Personality Sensitivity\n")
	for _, p := range []string{"Introvert", "Extrovert", "Seeker"} {
		retA := summaryA.RetentionByPersonality[p]
		retB := summaryB.RetentionByPersonality[p]
		scoreB := summaryB.ValidationByPersonality[p]
		fmt.Printf("  %-12s  retentionA=%.2fs  retentionB=%.2fs  validScore=%.1f\n",
			p, retA, retB, scoreB)
	}
}

func runScenario(s validation.Scenario, name string) metrics.Summary {
	fmt.Printf("Running %s...\n", name)
	rand.Seed(time.Now().UnixNano())

	eng := validation.NewEngine(s)
	rnk := ranker.New(SuppressionKappa, EchoChamberStrength)
	h := hub.New(eng, rnk, ActionBufSize)

	// Register agents — chia đều theo từng loại tính cách
	personalities := []agent.PersonalityType{agent.Introvert, agent.Extrovert, agent.Seeker}
	allAgents := make([]*agent.Agent, 0, NumAgentsPerType*3)

	for _, p := range personalities {
		for i := 0; i < NumAgentsPerType; i++ {
			a := h.RegisterAgentFmt(fmt.Sprintf("%03d", i), p)
			allAgents = append(allAgents, a)
		}
	}

	// Start hub
	quit := make(chan struct{})
	go h.Run(quit)

	// Start metrics collector
	col := metrics.New(h)
	go col.Collect(MetricsInterval, quit)

	// Launch agents
	for _, a := range allAgents {
		go a.Run()
	}

	// Run simulation
	time.Sleep(SimDuration)

	// Shutdown
	close(quit)

	// Give time for graceful shutdown
	time.Sleep(100 * time.Millisecond)

	// Stop all agents
	for _, a := range allAgents {
		a.Quit()
	}

	// Wait a bit for agents to finish
	time.Sleep(200 * time.Millisecond)

	return col.Summarise(name)
}

// Sẽ easter egg vào :]
