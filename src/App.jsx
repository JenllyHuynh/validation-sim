import { useState, useEffect, useRef, useCallback } from "react";
import Feed from "./components/Feed";
import DopamineBar from "./components/DopamineBar";
import StatsPanel from "./components/StatsPanel";
import PostComposer from "./components/PostComposer";
import PhaseReveal from "./components/PhaseReveal";
import HistoryPanel from "./components/HistoryPanel";
import "./App.css";

const WS_URL = "ws://localhost:8080/ws";

const PHASE = { BLIND: 1, CORRUPTION: 2, REVEAL: 3 };

export default function App() {
    const [wsStatus, setWsStatus] = useState("disconnected");
    const [phase, setPhase] = useState(PHASE.BLIND);
    const [activePct, setActivePct] = useState(0);
    const [dopamine, setDopamine] = useState({ current: 75, threshold: 30, maxLevel: 100, inflation: 0 });
    const [feedPosts, setFeedPosts] = useState([]);
    const [notifications, setNotifications] = useState([]);
    const [agentStats, setAgentStats] = useState(null);
    const [simRunning, setSimRunning] = useState(false);
    const [showRevealOverlay, setShowRevealOverlay] = useState(false);
    const [showHistory, setShowHistory] = useState(false);
    const [simSummary, setSimSummary] = useState(null);

    const wsRef = useRef(null);
    const activeStartRef = useRef(null);
    const activeTimerRef = useRef(null);
    const phaseRef = useRef(phase);
    phaseRef.current = phase;

    // Track active time để tính phase
    useEffect(() => {
        activeStartRef.current = Date.now();
        activeTimerRef.current = setInterval(() => {
            if (!simRunning) return;
            const elapsed = (Date.now() - activeStartRef.current) / 1000;
            // Sim duration = 8s per scenario × 2 = 16s total
            const pct = Math.min(elapsed / 16, 1.0);
            setActivePct(pct);

            const currentPhase = phaseRef.current;
            if (pct >= 0.5 && pct < 0.75 && currentPhase === PHASE.BLIND) {
                setPhase(PHASE.CORRUPTION);
            } else if (pct >= 0.75 && currentPhase === PHASE.CORRUPTION) {
                setPhase(PHASE.REVEAL);
                setShowRevealOverlay(true);
            }
        }, 200);
        return () => clearInterval(activeTimerRef.current);
    }, [simRunning]);

    // WebSocket connection
    const connectWS = useCallback(() => {
        if (wsRef.current?.readyState === WebSocket.OPEN) return;

        const ws = new WebSocket(WS_URL);
        wsRef.current = ws;

        ws.onopen = () => {
            setWsStatus("connected");
            console.log("WS connected");
        };

        ws.onclose = () => {
            setWsStatus("disconnected");
            // Auto reconnect sau 2s
            setTimeout(connectWS, 2000);
        };

        ws.onerror = () => setWsStatus("error");

        ws.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                handleWSMessage(msg);
            } catch (e) {
                console.error("WS parse error", e);
            }
        };
    }, []);

    useEffect(() => {
        connectWS();
        return () => wsRef.current?.close();
    }, [connectWS]);

    const handleWSMessage = useCallback((msg) => {
        switch (msg.type) {
            case "FEED_POST":
                setFeedPosts((prev) => [{ ...msg.payload, id: Date.now() + Math.random() }, ...prev].slice(0, 50));
                break;

            case "DOPAMINE_UPDATE":
                setDopamine(msg.payload);
                break;

            case "NOTIFICATION":
                const notif = { ...msg.payload, id: Date.now(), ts: msg.timestamp };
                setNotifications((prev) => [notif, ...prev].slice(0, 5));
                // Auto-dismiss sau 4s
                setTimeout(() => {
                    setNotifications((prev) => prev.filter((n) => n.id !== notif.id));
                }, 4000);
                break;

            case "PHASE_CHANGE":
                setPhase(msg.payload.phase);
                setActivePct(msg.payload.activePct);
                if (msg.payload.phase === PHASE.REVEAL) {
                    setShowRevealOverlay(true);
                }
                break;

            case "AGENT_STATS":
                setAgentStats(msg.payload);
                break;

            case "SIM_SUMMARY":
                setSimSummary(msg.payload);
                setSimRunning(false);
                break;

            default:
                break;
        }
    }, []);

    const startSimulation = async () => {
        try {
            await fetch("http://localhost:8080/api/simulate", { method: "POST" });
            setSimRunning(true);
            setFeedPosts([]);
            setNotifications([]);
            setActivePct(0);
            setPhase(PHASE.BLIND);
            setShowRevealOverlay(false);
            setSimSummary(null);
            activeStartRef.current = Date.now();
        } catch (e) {
            console.error("Failed to start simulation", e);
        }
    };

    const handleUserPost = (contentType, text) => {
        // Simulate human agent posting
        const isDeep = contentType === "deep";
        const fakeLikes = isDeep
            ? Math.floor(Math.random() * 8) + 1         // deep = ít like
            : Math.floor(Math.random() * 80) + 20;      // trend = nhiều like

        const post = {
            id: Date.now(),
            postId: `human-${Date.now()}`,
            authorId: "you",
            authorType: "human",
            contentType,
            content: text,
            likes: 0,
            targetLikes: fakeLikes,
            isEcho: false,
            reachPct: isDeep ? 0.1 : 0.8,
        };

        setFeedPosts((prev) => [post, ...prev].slice(0, 50));

        // Simulate dopamine response với delay
        const dopamineGain = isDeep
            ? Math.random() * 5 + 2    // deep = ít dopamine
            : Math.random() * 25 + 10; // trend = nhiều dopamine

        setTimeout(() => {
            setDopamine((prev) => ({
                ...prev,
                current: Math.min(prev.current + dopamineGain, 100),
            }));

            if (!isDeep && dopamineGain > 20) {
                setNotifications((prev) => [
                    {
                        id: Date.now(),
                        kind: "like_burst",
                        message: `+${Math.round(dopamineGain * 3)} người đã like bài của bạn! 🔥`,
                        points: dopamineGain,
                        isBurst: true,
                    },
                    ...prev,
                ].slice(0, 5));
            }
        }, isDeep ? 3000 : 800);
    };

    return (
        <div className="app" data-phase={phase}>
            {/* Corruption phase: subtle noise overlay */}
            {phase >= PHASE.CORRUPTION && <div className="corruption-veil" />}

            {/* Phase 3 Reveal Overlay */}
            {showRevealOverlay && (
                <PhaseReveal onDismiss={() => setShowRevealOverlay(false)} />
            )}

            {/* Notifications */}
            <div className="notif-stack">
                {notifications.map((n) => (
                    <div key={n.id} className={`notif ${n.isBurst ? "notif--burst" : ""}`}>
                        <span className="notif-icon">{n.kind === "top10" ? "🏆" : "🔥"}</span>
                        <span className="notif-msg">{n.message}</span>
                    </div>
                ))}
            </div>

            {/* Header */}
            <header className="app-header">
                <div className="header-left">
                    <span className="logo">val://sim</span>
                    <span className={`ws-dot ws-dot--${wsStatus}`} title={wsStatus} />
                </div>
                <div className="header-center">
                    {phase === PHASE.BLIND && <span className="phase-tag">● live</span>}
                    {phase === PHASE.CORRUPTION && <span className="phase-tag phase-tag--warn">● corrupting</span>}
                    {phase === PHASE.REVEAL && <span className="phase-tag phase-tag--reveal">● revealed</span>}
                </div>
                <div className="header-right">
                    <button className="btn-ghost" onClick={() => setShowHistory(!showHistory)}>
                        history
                    </button>
                    <button
                        className={`btn-primary ${simRunning ? "btn-primary--running" : ""}`}
                        onClick={startSimulation}
                        disabled={simRunning}
                    >
                        {simRunning ? "running..." : "run sim"}
                    </button>
                </div>
            </header>

            {/* Dopamine Bar - hidden until Phase 3 */}
            <DopamineBar
                dopamine={dopamine}
                phase={phase}
                activePct={activePct}
                visible={phase >= PHASE.REVEAL}
            />

            <main className="app-main">
                {/* Left: Post composer + Feed */}
                <div className="feed-col">
                    <PostComposer onPost={handleUserPost} phase={phase} />
                    <Feed posts={feedPosts} phase={phase} />
                </div>

                {/* Right: Stats panel */}
                <div className="stats-col">
                    <StatsPanel
                        agentStats={agentStats}
                        simRunning={simRunning}
                        phase={phase}
                        activePct={activePct}
                        simSummary={simSummary}
                    />
                </div>
            </main>

            {/* History Drawer */}
            {showHistory && <HistoryPanel onClose={() => setShowHistory(false)} />}
        </div>
    );
}