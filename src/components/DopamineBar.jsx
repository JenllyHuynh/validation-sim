import { useEffect, useRef, useState } from "react";

// DopamineBar — hidden until Phase 3 (75% active time)
// Khi xuất hiện, nó "reveal" trạng thái nội tâm mà người dùng
// đã bị ảnh hưởng mà không biết suốt quá trình trước đó
export default function DopamineBar({ dopamine, phase, activePct, visible }) {
    const { current = 75, threshold = 30, maxLevel = 100, inflation = 0 } = dopamine;
    const [animatedPct, setAnimatedPct] = useState(current);
    const [showInflation, setShowInflation] = useState(false);
    const prevInflation = useRef(inflation);
    const canvasRef = useRef(null);

    // Smooth animation
    useEffect(() => {
        const target = (current / maxLevel) * 100;
        const diff = target - animatedPct;
        if (Math.abs(diff) < 0.1) return;
        const frame = requestAnimationFrame(() => {
            setAnimatedPct((p) => p + diff * 0.12);
        });
        return () => cancelAnimationFrame(frame);
    }, [current, maxLevel, animatedPct]);

    // Flash inflation indicator when ET rises
    useEffect(() => {
        if (inflation > prevInflation.current) {
            setShowInflation(true);
            setTimeout(() => setShowInflation(false), 2000);
        }
        prevInflation.current = inflation;
    }, [inflation]);

    // Draw sparkline history on canvas
    const historyRef = useRef([]);
    useEffect(() => {
        historyRef.current = [...historyRef.current, current].slice(-80);
        const canvas = canvasRef.current;
        if (!canvas || !visible) return;
        const ctx = canvas.getContext("2d");
        const w = canvas.width;
        const h = canvas.height;
        ctx.clearRect(0, 0, w, h);

        const data = historyRef.current;
        if (data.length < 2) return;

        ctx.beginPath();
        ctx.strokeStyle = pctColor(animatedPct);
        ctx.lineWidth = 1.5;
        ctx.shadowBlur = 6;
        ctx.shadowColor = pctColor(animatedPct);

        data.forEach((v, i) => {
            const x = (i / (data.length - 1)) * w;
            const y = h - (v / 100) * h;
            i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
        });
        ctx.stroke();
    }, [current, visible, animatedPct]);

    const pct = animatedPct;
    const thresholdPct = (threshold / maxLevel) * 100;
    const color = pctColor(pct);
    const status = pct < thresholdPct * 0.5
        ? "critical"
        : pct < thresholdPct
            ? "low"
            : pct < 60
                ? "normal"
                : "high";

    if (!visible) {
        // Phase 1 & 2: ẩn hoàn toàn, chỉ render một placeholder invisible
        return <div className="dopamine-bar dopamine-bar--hidden" aria-hidden="true" />;
    }

    return (
        <div className="dopamine-bar dopamine-bar--visible">
            <div className="dbar-inner">
                {/* Label */}
                <div className="dbar-label">
                    <span className="dbar-title">dopamine</span>
                    <span className="dbar-status" data-status={status}>
            {statusLabel(status)}
          </span>
                    {showInflation && (
                        <span className="dbar-inflation">
              ↑ ngưỡng tăng ({(inflation * 100).toFixed(0)}%)
            </span>
                    )}
                </div>

                {/* Main bar */}
                <div className="dbar-track">
                    {/* Threshold marker */}
                    <div
                        className="dbar-threshold-marker"
                        style={{ left: `${thresholdPct}%` }}
                        title={`ET: ${threshold.toFixed(1)}`}
                    />
                    {/* Fill */}
                    <div
                        className="dbar-fill"
                        style={{
                            width: `${Math.max(0, Math.min(100, pct))}%`,
                            "--bar-color": color,
                        }}
                    />
                    {/* Value label */}
                    <span className="dbar-value">{current.toFixed(1)}</span>
                </div>

                {/* Sparkline */}
                <canvas
                    ref={canvasRef}
                    className="dbar-sparkline"
                    width={320}
                    height={32}
                />

                {/* ET inflation indicator */}
                <div className="dbar-meta">
                    <span>ET: {threshold.toFixed(1)}</span>
                    <span>inflation ×{(1 + inflation).toFixed(2)}</span>
                    <span>phase {phase}/3</span>
                </div>
            </div>

            {/* Reveal message — chỉ hiện ngay lúc phase 3 bắt đầu */}
            <div className="dbar-reveal-msg">
                <span>Đây là trạng thái nội tâm của bạn trong suốt thời gian qua.</span>
            </div>
        </div>
    );
}

function pctColor(pct) {
    if (pct < 20) return "#ef4444";
    if (pct < 40) return "#f97316";
    if (pct < 65) return "#eab308";
    return "#22c55e";
}

function statusLabel(s) {
    return { critical: "⚠ cạn kiệt", low: "↓ thấp", normal: "● ổn định", high: "↑ đủ đầy" }[s];
}