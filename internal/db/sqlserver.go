package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// DB wraps SQL Server connection
type DB struct {
	conn *sql.DB
}

// Config - đọc từ env hoặc hardcode khi dev
type Config struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
}

func Open(cfg Config) (*DB, error) {
	var connStr string

	if cfg.User == "" {
		connStr = fmt.Sprintf(
			"server=%s;port=%d;database=%s;integrated security=true;encrypt=disable",
			cfg.Host, cfg.Port, cfg.Database,
		)
	} else {
		// SQL Auth
		connStr = fmt.Sprintf(
			"server=%s;port=%d;database=%s;user id=%s;password=%s;encrypt=disable",
			cfg.Host, cfg.Port, cfg.Database, cfg.User, cfg.Password,
		)
	}

	conn, err := sql.Open("sqlserver", connStr)
	if err != nil {
		return nil, fmt.Errorf("open mssql: %w", err)
	}

	// Ping để kiểm tra kết nối ngay
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping mssql (check host/auth): %w", err)
	}

	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	log.Printf("Connected to SQL Server: %s/%s", cfg.Host, cfg.Database)
	return &DB{conn: conn}, nil
}

//Structs

type ScenarioResult struct {
	MeanRetentionSec        float64            `json:"meanRetentionSec"`
	MeanCQDI                float64            `json:"meanCqdi"`
	MeanValidationScore     float64            `json:"meanValidationScore"`
	DisengagementRate       float64            `json:"disengagementRate"`
	RetentionByPersonality  map[string]float64 `json:"retentionByPersonality"`
	ValidationByPersonality map[string]float64 `json:"validationByPersonality"`
}

type SessionRecord struct {
	ID             int64
	RunAt          time.Time
	ScenarioA      ScenarioResult
	ScenarioB      ScenarioResult
	DeltaRetention float64
	DeltaCQDI      float64
	NumAgents      int
	DurationSec    float64
}

type HumanSessionRecord struct {
	ID              int64
	SessionID       int64
	RunAt           time.Time
	FinalDopamine   float64
	FinalScore      float64
	InflationFactor float64
	ActiveTimeSec   float64
	PostCount       int
	LowEffortRatio  float64
	ReachedPhase2   bool
	ReachedPhase3   bool
	Phase2AtPct     float64
	Phase3AtPct     float64
}

// Session operations

// SaveSession lưu kết quả một lần chạy simulation
func (d *DB) SaveSession(a, b ScenarioResult, numAgents int, durationSec float64) (int64, error) {
	retentionJSON, _ := json.Marshal(b.RetentionByPersonality)
	validJSON, _ := json.Marshal(b.ValidationByPersonality)

	delta := b.MeanRetentionSec - a.MeanRetentionSec
	deltaCQDI := b.MeanCQDI - a.MeanCQDI

	// OUTPUT INSERTED.id để lấy identity value từ SQL Server
	row := d.conn.QueryRow(`
		INSERT INTO simulation_sessions (
			num_agents, duration_sec,
			a_mean_retention, a_mean_cqdi, a_mean_valid_score, a_disengagement_pct,
			b_mean_retention, b_mean_cqdi, b_mean_valid_score, b_disengagement_pct,
			delta_retention, delta_cqdi,
			retention_by_personality_json, validation_by_personality_json
		)
		OUTPUT INSERTED.id
		VALUES (
			@p1, @p2,
			@p3, @p4, @p5, @p6,
			@p7, @p8, @p9, @p10,
			@p11, @p12,
			@p13, @p14
		)`,
		numAgents, durationSec,
		a.MeanRetentionSec, a.MeanCQDI, a.MeanValidationScore, a.DisengagementRate,
		b.MeanRetentionSec, b.MeanCQDI, b.MeanValidationScore, b.DisengagementRate,
		delta, deltaCQDI,
		string(retentionJSON), string(validJSON),
	)

	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, fmt.Errorf("save session: %w", err)
	}
	log.Printf("Session saved id=%d | Δretention=%.2fs | ΔCQDI=%.3f", id, delta, deltaCQDI)
	return id, nil
}

// SaveHumanSession lưu kết quả human agent sau mỗi lần chơi
func (d *DB) SaveHumanSession(rec HumanSessionRecord) (int64, error) {
	p2, p3 := 0, 0
	if rec.ReachedPhase2 {
		p2 = 1
	}
	if rec.ReachedPhase3 {
		p3 = 1
	}

	row := d.conn.QueryRow(`
		INSERT INTO human_sessions (
			session_id, final_dopamine, final_score, inflation_factor,
			active_time_sec, post_count, low_effort_ratio,
			reached_phase2, reached_phase3, phase2_at_pct, phase3_at_pct
		)
		OUTPUT INSERTED.id
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11)`,
		rec.SessionID, rec.FinalDopamine, rec.FinalScore, rec.InflationFactor,
		rec.ActiveTimeSec, rec.PostCount, rec.LowEffortRatio,
		p2, p3, rec.Phase2AtPct, rec.Phase3AtPct,
	)

	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, fmt.Errorf("save human session: %w", err)
	}
	return id, nil
}

// Query operations

// GetRecentSessions trả về N lần chạy gần nhất
func (d *DB) GetRecentSessions(limit int) ([]SessionRecord, error) {
	rows, err := d.conn.Query(`
		SELECT TOP (@p1)
			id, run_at,
			a_mean_retention, a_mean_cqdi, a_mean_valid_score, a_disengagement_pct,
			b_mean_retention, b_mean_cqdi, b_mean_valid_score, b_disengagement_pct,
			delta_retention, delta_cqdi, num_agents, duration_sec,
			retention_by_personality_json, validation_by_personality_json
		FROM simulation_sessions
		ORDER BY run_at DESC`, limit)
	if err != nil {
		return nil, fmt.Errorf("get sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionRecord
	for rows.Next() {
		var s SessionRecord
		var retJSON, valJSON sql.NullString
		err := rows.Scan(
			&s.ID, &s.RunAt,
			&s.ScenarioA.MeanRetentionSec, &s.ScenarioA.MeanCQDI,
			&s.ScenarioA.MeanValidationScore, &s.ScenarioA.DisengagementRate,
			&s.ScenarioB.MeanRetentionSec, &s.ScenarioB.MeanCQDI,
			&s.ScenarioB.MeanValidationScore, &s.ScenarioB.DisengagementRate,
			&s.DeltaRetention, &s.DeltaCQDI, &s.NumAgents, &s.DurationSec,
			&retJSON, &valJSON,
		)
		if err != nil {
			log.Printf("scan session row: %v", err)
			continue
		}
		if retJSON.Valid {
			json.Unmarshal([]byte(retJSON.String), &s.ScenarioB.RetentionByPersonality)
		}
		if valJSON.Valid {
			json.Unmarshal([]byte(valJSON.String), &s.ScenarioB.ValidationByPersonality)
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// GetHumanHistory trả về lịch sử human sessions
func (d *DB) GetHumanHistory(limit int) ([]HumanSessionRecord, error) {
	rows, err := d.conn.Query(`
		SELECT TOP (@p1)
			id, ISNULL(session_id, 0), run_at,
			final_dopamine, final_score, inflation_factor,
			active_time_sec, post_count, low_effort_ratio,
			reached_phase2, reached_phase3,
			ISNULL(phase2_at_pct, 0), ISNULL(phase3_at_pct, 0)
		FROM human_sessions
		ORDER BY run_at DESC`, limit)
	if err != nil {
		return nil, fmt.Errorf("get human history: %w", err)
	}
	defer rows.Close()

	var records []HumanSessionRecord
	for rows.Next() {
		var r HumanSessionRecord
		var p2, p3 int
		err := rows.Scan(
			&r.ID, &r.SessionID, &r.RunAt,
			&r.FinalDopamine, &r.FinalScore, &r.InflationFactor,
			&r.ActiveTimeSec, &r.PostCount, &r.LowEffortRatio,
			&p2, &p3, &r.Phase2AtPct, &r.Phase3AtPct,
		)
		if err != nil {
			log.Printf("scan human row: %v", err)
			continue
		}
		r.ReachedPhase2 = p2 == 1
		r.ReachedPhase3 = p3 == 1
		records = append(records, r)
	}
	return records, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}
