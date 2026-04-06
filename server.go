//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"validation-sim/internal/agent"
	appdb "validation-sim/internal/db"
	"validation-sim/internal/hub"
	"validation-sim/internal/metrics"
	"validation-sim/internal/ranker"
	"validation-sim/internal/validation"
	ws "validation-sim/internal/websocket"

	"github.com/joho/godotenv"
)

const (
	NumAgentsPerType    = 100
	SimDuration         = 8 * time.Second
	MetricsInterval     = 200 * time.Millisecond
	ActionBufSize       = 4096
	SuppressionKappa    = 0.7
	EchoChamberStrength = 0.65
	ServerPort          = ":8080"
)

func dbConfig() appdb.Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from system env")
	}

	port := 1433
	if p := os.Getenv("DB_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "validation_sim"
	}
	return appdb.Config{
		Host:     host,
		Port:     port,
		Database: dbName,
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"), // khớp với .env
	}
}

var deepTemplates = []string{
	"Mình vừa đọc xong một nghiên cứu về dopamine loop trong mạng xã hội...",
	"Suy nghĩ về việc thuật toán đang định hình cách chúng ta tư duy",
	"Thread dài: Tại sao nội dung chất lượng ngày càng bị chôn vùi?",
	"Góc nhìn khác về echo chamber và sự tha hóa của nội dung online",
}

var trendTemplates = []string{
	"POV: bạn là người duy nhất không biết trend này",
	"ratio + W + không có lý do gì cả",
	"bro discovered water",
	"main character era không?",
	"aura farming",
	"ngl đây là lần đầu tiên mình đồng ý 100%",
}

func randomContent(ct agent.ContentType) string {
	if ct == agent.DeepContent {
		return deepTemplates[rand.Intn(len(deepTemplates))]
	}
	return trendTemplates[rand.Intn(len(trendTemplates))]
}

func main() {
	rand.Seed(time.Now().UnixNano())
	fmt.Println("Validation Simulation — Web Mode")
	fmt.Printf("Connecting to SQL Server...\n")

	database, err := appdb.Open(dbConfig())
	if err != nil {
		log.Fatalf("DB failed: %v\n\nSet env vars:\n  DB_HOST=localhost\\SQLEXPRESS\n  DB_NAME=validation_sim\nHoặc xem README để biết thêm.\n", err)
	}
	defer database.Close()

	wsHub := ws.NewHub()
	go wsHub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHub.ServeWS)

	mux.HandleFunc("/api/simulate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		go runAndBroadcast(wsHub, database)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "started"})
	})

	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		sessions, err := database.GetRecentSessions(20)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	})

	mux.HandleFunc("/api/history/human", func(w http.ResponseWriter, r *http.Request) {
		records, err := database.GetHumanHistory(20)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	})

	fmt.Printf("Server: http://localhost%s\n", ServerPort)
	fmt.Printf("WebSocket: ws://localhost%s/ws\n", ServerPort)
	log.Fatal(http.ListenAndServe(ServerPort, corsMiddleware(mux)))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(200)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func runAndBroadcast(wsHub *ws.Hub, database *appdb.DB) {
	start := time.Now()
	summaryA := runScenario(validation.ScenarioA, "Scenario A (linear)", wsHub)
	summaryB := runScenario(validation.ScenarioB, "Scenario B (variable-ratio)", wsHub)

	aResult := appdb.ScenarioResult{
		MeanRetentionSec:        summaryA.MeanRetentionSec,
		MeanCQDI:                summaryA.MeanCQDI,
		MeanValidationScore:     summaryA.MeanValidationScore,
		DisengagementRate:       summaryA.DisengagementRate,
		RetentionByPersonality:  summaryA.RetentionByPersonality,
		ValidationByPersonality: summaryA.ValidationByPersonality,
	}
	bResult := appdb.ScenarioResult{
		MeanRetentionSec:        summaryB.MeanRetentionSec,
		MeanCQDI:                summaryB.MeanCQDI,
		MeanValidationScore:     summaryB.MeanValidationScore,
		DisengagementRate:       summaryB.DisengagementRate,
		RetentionByPersonality:  summaryB.RetentionByPersonality,
		ValidationByPersonality: summaryB.ValidationByPersonality,
	}

	sessionID, _ := database.SaveSession(aResult, bResult, NumAgentsPerType*3, time.Since(start).Seconds())

	wsHub.Broadcast(ws.MsgSimSummary, map[string]interface{}{
		"sessionId": sessionID,
		"scenarioA": aResult,
		"scenarioB": bResult,
		"delta": map[string]float64{
			"retention": bResult.MeanRetentionSec - aResult.MeanRetentionSec,
			"cqdi":      bResult.MeanCQDI - aResult.MeanCQDI,
		},
	})
}

func runScenario(s validation.Scenario, name string, wsHub *ws.Hub) metrics.Summary {
	fmt.Printf("Running %s...\n", name)
	eng := validation.NewEngine(s)
	rnk := ranker.New(SuppressionKappa, EchoChamberStrength)
	h := hub.New(eng, rnk, ActionBufSize)

	personalities := []agent.PersonalityType{agent.Introvert, agent.Extrovert, agent.Seeker}
	allAgents := make([]*agent.Agent, 0, NumAgentsPerType*3)
	for _, p := range personalities {
		for i := 0; i < NumAgentsPerType; i++ {
			a := h.RegisterAgentFmt(fmt.Sprintf("%03d", i), p)
			allAgents = append(allAgents, a)
		}
	}

	quit := make(chan struct{})
	go h.Run(quit)

	col := metrics.New(h)
	go col.Collect(MetricsInterval, quit)
	go broadcastStats(wsHub, h, quit)
	go broadcastFeed(wsHub, h, quit)

	for _, a := range allAgents {
		go a.Run()
	}

	time.Sleep(SimDuration)
	close(quit)
	time.Sleep(100 * time.Millisecond)
	for _, a := range allAgents {
		a.Quit()
	}
	time.Sleep(200 * time.Millisecond)
	return col.Summarise(name)
}

func broadcastStats(wsHub *ws.Hub, h *hub.Hub, quit <-chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			ids := h.AllAgentIDs()
			stats := ws.AgentStats{TotalAgents: len(ids), ByPersonality: make(map[string]ws.PersonalStat)}
			totalDop := 0.0
			trendPosts, totalPosts := 0, 0
			for _, id := range ids {
				a, ok := h.AgentByID(id)
				if !ok || a == nil {
					continue
				}
				a.Lock()
				p := a.Personality.String()
				ps := stats.ByPersonality[p]
				ps.Count++
				ps.MeanDopamine += a.CurrentDopamine
				ps.MeanScore += a.ValidationScore
				totalDop += a.CurrentDopamine
				totalPosts += a.PostCount
				trendPosts += int(float64(a.PostCount) * a.LowEffortRatio)
				if a.CurrentDopamine < a.EngagementThreshold*0.25 {
					ps.Disengaged++
					stats.DisengagedCount++
				}
				a.Unlock()
				stats.ByPersonality[p] = ps
			}
			if len(ids) > 0 {
				stats.MeanDopamine = totalDop / float64(len(ids))
			}
			if totalPosts > 0 {
				stats.TrendPostPct = float64(trendPosts) / float64(totalPosts) * 100
			}
			for p, ps := range stats.ByPersonality {
				if ps.Count > 0 {
					ps.MeanDopamine /= float64(ps.Count)
					ps.MeanScore /= float64(ps.Count)
				}
				stats.ByPersonality[p] = ps
			}
			wsHub.Broadcast(ws.MsgAgentStats, stats)
		}
	}
}

func broadcastFeed(wsHub *ws.Hub, h *hub.Hub, quit <-chan struct{}) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	lastLen := 0
	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			h.RewardLogMu.Lock()
			newEntries := h.RewardLog[lastLen:]
			lastLen = len(h.RewardLog)
			h.RewardLogMu.Unlock()
			for _, rec := range newEntries {
				ct := "deep"
				if rec.ContentType == agent.TrendContent {
					ct = "trend"
				}
				likes := rand.Intn(5) + 1
				if rec.ContentType == agent.TrendContent {
					likes = rand.Intn(50) + 10
				}
				wsHub.Broadcast(ws.MsgFeedPost, ws.FeedPost{
					PostID:      fmt.Sprintf("post-%d", rand.Int()),
					AuthorID:    rec.AgentID,
					AuthorType:  "agent",
					ContentType: ct,
					Content:     randomContent(rec.ContentType),
					Likes:       likes,
					ReachPct:    rec.Points / 20,
				})
			}
		}
	}
}
