# val://sim - Validation Simulation (v1.0)

**A/B experiment: Thuật toán validation định hình hành vi người dùng như thế nào?**

Một mô phỏng khoa học xã hội + social media algorithm được viết hoàn toàn bằng **Go** (backend) + **React + Vite** (frontend). Dự án tái hiện cơ chế **dopamine loop**, **content suppression**, **echo chamber** và **variable-ratio reward** (gacha-like) mà các nền tảng lớn đang sử dụng.

---

## Mục tiêu nghiên cứu (Hypotheses)

- **H1**: Variable-ratio reward (Scenario B) **tăng retention** đáng kể so với linear reward (Scenario A).
- **H2**: Variable-ratio reward gây **Content Quality Drift Index (CQDI)** cao hơn -> người dùng dần chuyển sang nội dung low-effort / trend.
- **H3**: Mức độ nhạy cảm khác nhau theo **personality** (Introvert < Extrovert < Seeker).

---

## Tính năng nổi bật

- **300 agents** (100 Introvert, 100 Extrovert, 100 Seeker) chạy song song dưới dạng goroutines.
- Hai kịch bản A/B:
    - **Scenario A** (control): Linear reward - thưởng đều mỗi 5 actions.
    - **Scenario B** (treatment): Variable-ratio + dopamine-dependent burst reward.
- **Dopamine Inflation**: Engagement Threshold tăng dần sau mỗi burst/top-10% notification.
- **Human agent** (bạn) có thể đăng bài deep/trend và quan sát dopamine cá nhân.
- **3 phases** trải nghiệm mù:
    - Phase 1 - *Blind*: tương tác bình thường, không có thông tin gì.
    - Phase 2 - *Corruption*: deep content bắt đầu bị mờ dần, trend được đẩy lên.
    - Phase 3 - *Reveal*: Dopamine Bar xuất hiện, reveal trạng thái nội tâm.
- Live WebSocket feed + real-time stats + notifications.
- Lưu toàn bộ kết quả vào **SQL Server** (schema đầy đủ 6 bảng).
- Giao diện **dark brutalist** - IBM Plex Mono + Space Mono.

---

## Tech Stack

**Backend (Go)**
- Go 1.21+
- Goroutine-based simulation engine (300 concurrent agents)
- WebSocket - `gorilla/websocket`
- SQL Server - `microsoft/go-mssqldb`
- `.env` - `joho/godotenv`

**Frontend**
- React 18 + Vite
- Pure CSS (không dùng Tailwind hay UI lib)
- WebSocket native API

**Database**
- SQL Server 2016+
- 6 bảng: `users`, `simulation_sessions`, `posts`, `interactions`, `simulation_metrics`, `human_sessions`

---

## Cấu trúc dự án

```
validation-sim/
├── main.go                  # CLI mode - in số liệu ra terminal
├── server.go                # Web mode - HTTP + WebSocket + SQL Server
├── .env                     # Database config (không commit lên git)
├── schema.sql               # Chạy trên SSMS trước khi dùng web mode
├── go.mod
│
├── internal/
│   ├── agent/
│   │   ├── agent.go         # Goroutine tick logic
│   │   └── types.go         # State + Dopamine Inflation
│   ├── db/
│   │   └── sqlserver.go     # SQL Server repository
│   ├── hub/
│   │   └── hub.go           # Central message router
│   ├── metrics/
│   │   └── collector.go     # Snapshot 200ms + summary
│   ├── ranker/
│   │   └── ranker.go        # Content suppression (κ=0.7) + echo chamber
│   ├── validation/
│   │   └── engine.go        # Scenario A/B reward engine
│   └── websocket/
│       └── hub.go           # WS broadcast hub
│
├── src/                     # React frontend
│   ├── App.jsx
│   ├── App.css              # Brutalist dark theme
│   ├── main.jsx
│   └── components/
│       ├── DopamineBar.jsx  # Hidden -> visible tại phase 3
│       ├── Feed.jsx         # Social feed, phase-aware
│       └── index.jsx        # PostComposer, StatsPanel, PhaseReveal, HistoryPanel
│
├── index.html
├── package.json
└── vite.config.js
```

---

## Cách chạy

### Bước 1 - Database (chỉ cần làm 1 lần)

Mở `schema.sql` trong SSMS -> Execute.

Tạo file `.env` ở root project:

```env
DB_HOST=localhost
DB_PORT=1433
DB_NAME=validation_sim
DB_USER=sa
DB_PASSWORD=yourpassword
```

### Bước 2a - CLI mode (không cần DB, không cần React)

```bash
go run main.go
```

### Bước 2b - Web mode

Terminal 1 - Go backend:
```bash
go mod tidy
go run server.go
# Server: http://localhost:8080
# WebSocket: ws://localhost:8080/ws
```

Terminal 2 - React frontend:
```bash
npm install
npm run dev
# UI: http://localhost:3000
```

Mở `http://localhost:3000`, bấm **"run sim"** và trải nghiệm

### Build production

```bash
# Backend
go build -o val-sim server.go

# Frontend
npm run build
```

---

## Kết quả điển hình

Sau 8 giây simulation với 300 agents:

```
H1 TEST: Retention Δ (B - A)
Mean retention increase: +1.14s  (+14.2%)

H2 TEST: Content Quality Drift
Scenario A CQDI: 0.547  |  Scenario B CQDI: 0.419

H3 TEST: Personality Sensitivity
  Introvert     retentionA=6.20s  retentionB=6.85s  validScore=178.0
  Extrovert     retentionA=6.91s  retentionB=7.43s  validScore=232.0
  Seeker        retentionA=5.80s  retentionB=9.01s  validScore=298.0
```

> Kết quả H2 ngược intuition: Scenario B tạo ra **ít** low-effort content hơn A
> Lý giải: burst reward trong B giữ dopamine ổn định hơn -> agent ít rơi vào trạng thái "đói" -> ít cần post trend để cứu vãn dopamine
