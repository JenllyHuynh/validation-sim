// PostComposer — nơi human agent đăng bài
import { useState } from "react";

const PHASE = { BLIND: 1, CORRUPTION: 2, REVEAL: 3 };

export function PostComposer({ onPost, phase }) {
    const [text, setText] = useState("");
    const [contentType, setContentType] = useState("deep");

    const handleSubmit = () => {
        if (!text.trim()) return;
        onPost(contentType, text.trim());
        setText("");
    };

    return (
        <div className="composer">
            <div className="composer-tabs">
                <button
                    className={`composer-tab ${contentType === "deep" ? "composer-tab--active" : ""}`}
                    onClick={() => setContentType("deep")}
                >
                    deep
                </button>
                <button
                    className={`composer-tab ${contentType === "trend" ? "composer-tab--active" : ""}`}
                    onClick={() => setContentType("trend")}
                >
                    trend ✨
                </button>
            </div>
            <textarea
                className="composer-input"
                placeholder={
                    contentType === "deep"
                        ? "Chia sẻ suy nghĩ thật của bạn..."
                        : "Viết gì đó theo trend..."
                }
                value={text}
                onChange={(e) => setText(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && e.ctrlKey && handleSubmit()}
                rows={3}
            />
            <div className="composer-footer">
                <span className="composer-hint">Ctrl+Enter để đăng</span>
                <button className="composer-submit" onClick={handleSubmit} disabled={!text.trim()}>
                    đăng bài
                </button>
            </div>
        </div>
    );
}

// StatsPanel — hiển thị trạng thái simulation
export function StatsPanel({ agentStats, simRunning, phase, activePct, simSummary }) {
    const showFull = phase >= PHASE.REVEAL;

    return (
        <div className="stats-panel">
            <div className="stats-header">
                <span className="stats-title">simulation</span>
                {simRunning && <span className="stats-pulse">●</span>}
            </div>

            {/* Progress */}
            <div className="stats-progress">
                <div className="stats-progress-label">
                    <span>progress</span>
                    <span>{(activePct * 100).toFixed(0)}%</span>
                </div>
                <div className="stats-progress-track">
                    <div className="stats-progress-fill" style={{ width: `${activePct * 100}%` }} />
                    <div className="stats-phase-marker" style={{ left: "50%" }} title="Phase 2" />
                    <div className="stats-phase-marker" style={{ left: "75%" }} title="Phase 3" />
                </div>
            </div>

            {agentStats && (
                <>
                    <div className="stats-row">
                        <span className="stats-label">agents</span>
                        <span className="stats-value">{agentStats.totalAgents}</span>
                    </div>
                    <div className="stats-row">
                        <span className="stats-label">mean dopamine</span>
                        <span className="stats-value">{agentStats.meanDopamine?.toFixed(1)}</span>
                    </div>
                    <div className="stats-row">
                        <span className="stats-label">disengaged</span>
                        <span className="stats-value stats-value--warn">{agentStats.disengagedCount}</span>
                    </div>
                    <div className="stats-row">
                        <span className="stats-label">trend posts</span>
                        <span className="stats-value">{agentStats.trendPostPct?.toFixed(1)}%</span>
                    </div>

                    {/* Phase 3: personality breakdown */}
                    {showFull && agentStats.byPersonality && (
                        <div className="stats-personality">
                            <div className="stats-section-title">by personality</div>
                            {Object.entries(agentStats.byPersonality).map(([p, s]) => (
                                <div key={p} className="stats-personality-row">
                                    <span className="stats-p-label">{p}</span>
                                    <div className="stats-p-bar-wrap">
                                        <div
                                            className="stats-p-bar"
                                            style={{ width: `${s.meanDopamine}%` }}
                                            data-personality={p.toLowerCase()}
                                        />
                                    </div>
                                    <span className="stats-p-val">{s.meanDopamine?.toFixed(0)}</span>
                                </div>
                            ))}
                        </div>
                    )}
                </>
            )}

            {/* Simulation summary */}
            {simSummary && (
                <div className="stats-summary">
                    <div className="stats-section-title">H1 — retention</div>
                    <div className="stats-delta">
                        Δ +{simSummary.delta?.retention?.toFixed(2)}s
                        ({simSummary.scenarioA?.meanRetentionSec > 0
                        ? ((simSummary.delta?.retention / simSummary.scenarioA?.meanRetentionSec) * 100).toFixed(1)
                        : 0}%)
                    </div>
                    <div className="stats-section-title">H2 — CQDI</div>
                    <div className="stats-row">
                        <span>A: {simSummary.scenarioA?.meanCqdi?.toFixed(3)}</span>
                        <span>B: {simSummary.scenarioB?.meanCqdi?.toFixed(3)}</span>
                    </div>
                </div>
            )}

            {!agentStats && !simRunning && (
                <div className="stats-idle">
                    <span>chưa có dữ liệu — bấm "run sim"</span>
                </div>
            )}
        </div>
    );
}

// PhaseReveal — overlay moment khi Dopamine Bar xuất hiện lần đầu
export function PhaseReveal({ onDismiss }) {
    return (
        <div className="phase-reveal-overlay" onClick={onDismiss}>
            <div className="phase-reveal-modal" onClick={(e) => e.stopPropagation()}>
                <div className="phase-reveal-icon">◉</div>
                <h2 className="phase-reveal-title">Đây là những gì thật sự đang xảy ra.</h2>
                <p className="phase-reveal-body">
                    Trong suốt thời gian qua, mức dopamine của bạn đã dao động theo từng
                    tương tác — tăng khi nhận được like, tụt dần khi im lặng.
                </p>
                <p className="phase-reveal-body">
                    Thuật toán biết chính xác lúc nào bạn đang "đói" nhất để tung ra
                    phần thưởng — giữ bạn ở lại lâu hơn bạn định.
                </p>
                <button className="phase-reveal-btn" onClick={onDismiss}>
                    tôi hiểu rồi
                </button>
            </div>
        </div>
    );
}

// HistoryPanel — drawer hiển thị lịch sử các lần chạy
export function HistoryPanel({ onClose }) {
    const [sessions, setSessions] = useState([]);
    const [loading, setLoading] = useState(true);

    useState(() => {
        fetch("http://localhost:8080/api/history")
            .then((r) => r.json())
            .then((data) => {
                setSessions(data || []);
                setLoading(false);
            })
            .catch(() => setLoading(false));
    });

    return (
        <div className="history-overlay" onClick={onClose}>
            <div className="history-drawer" onClick={(e) => e.stopPropagation()}>
                <div className="history-header">
                    <span className="history-title">session history</span>
                    <button className="history-close" onClick={onClose}>✕</button>
                </div>

                {loading ? (
                    <div className="history-loading">loading...</div>
                ) : sessions.length === 0 ? (
                    <div className="history-empty">Chưa có lần chạy nào được lưu.</div>
                ) : (
                    <div className="history-list">
                        {sessions.map((s) => (
                            <div key={s.ID} className="history-item">
                                <div className="history-item-header">
                                    <span className="history-item-id">#{s.ID}</span>
                                    <span className="history-item-date">
                    {new Date(s.RunAt).toLocaleString("vi-VN")}
                  </span>
                                </div>
                                <div className="history-item-stats">
                                    <div className="history-stat">
                                        <span className="history-stat-label">Δ retention</span>
                                        <span className={`history-stat-val ${s.DeltaRetention > 0 ? "pos" : "neg"}`}>
                      {s.DeltaRetention > 0 ? "+" : ""}{s.DeltaRetention?.toFixed(2)}s
                    </span>
                                    </div>
                                    <div className="history-stat">
                                        <span className="history-stat-label">Δ CQDI</span>
                                        <span className="history-stat-val">{s.DeltaCQDI?.toFixed(3)}</span>
                                    </div>
                                    <div className="history-stat">
                                        <span className="history-stat-label">agents</span>
                                        <span className="history-stat-val">{s.NumAgents}</span>
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </div>
    );
}