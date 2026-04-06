import { useEffect, useRef, useState } from "react";

const PHASE = { BLIND: 1, CORRUPTION: 2, REVEAL: 3 };

export default function Feed({ posts, phase }) {
    const feedRef = useRef(null);
    const [likeAnimations, setLikeAnimations] = useState({});

    // Auto-scroll to top khi có post mới
    useEffect(() => {
        if (feedRef.current) {
            feedRef.current.scrollTo({ top: 0, behavior: "smooth" });
        }
    }, [posts.length]);

    const handleLike = (postId) => {
        setLikeAnimations((prev) => ({ ...prev, [postId]: true }));
        setTimeout(() => {
            setLikeAnimations((prev) => ({ ...prev, [postId]: false }));
        }, 600);
    };

    if (posts.length === 0) {
        return (
            <div className="feed feed--empty">
                <div className="feed-empty-state">
                    <span className="feed-empty-icon">◌</span>
                    <p>Feed trống — bấm "run sim" để bắt đầu</p>
                </div>
            </div>
        );
    }

    return (
        <div className="feed" ref={feedRef}>
            {posts.map((post) => (
                <FeedCard
                    key={post.id}
                    post={post}
                    phase={phase}
                    likeAnimating={likeAnimations[post.postId]}
                    onLike={() => handleLike(post.postId)}
                />
            ))}
        </div>
    );
}

function FeedCard({ post, phase, likeAnimating, onLike }) {
    const isDeep = post.contentType === "deep";
    const isHuman = post.authorType === "human";
    const [likes, setLikes] = useState(post.likes || 0);

    // Simulate likes trickling in
    useEffect(() => {
        if (post.targetLikes && post.targetLikes > likes) {
            const interval = setInterval(() => {
                setLikes((l) => {
                    if (l >= post.targetLikes) {
                        clearInterval(interval);
                        return l;
                    }
                    return l + Math.ceil(Math.random() * 3);
                });
            }, isDeep ? 800 : 200);
            return () => clearInterval(interval);
        }
    }, [post.targetLikes]);

    // Phase 2: Deep content cards subtly muted
    const cardMuted = phase >= PHASE.CORRUPTION && isDeep && !isHuman;

    // Phase 3: Show reach indicator
    const showReach = phase >= PHASE.REVEAL;

    return (
        <article
            className={`feed-card ${isDeep ? "feed-card--deep" : "feed-card--trend"} ${isHuman ? "feed-card--human" : ""} ${cardMuted ? "feed-card--muted" : ""}`}
        >
            <div className="card-header">
                <div className="card-avatar">
                    {isHuman ? "👤" : post.authorType === "agent" ? agentEmoji(post.authorId) : "◎"}
                </div>
                <div className="card-meta">
          <span className="card-author">
            {isHuman ? "you" : post.authorId?.slice(0, 12) + "…"}
          </span>
                    <span className="card-type-badge" data-type={post.contentType}>
            {isDeep ? "deep" : "trend"}
          </span>
                </div>
            </div>

            <p className="card-content">{post.content}</p>

            <div className="card-footer">
                <button
                    className={`card-like-btn ${likeAnimating ? "card-like-btn--active" : ""}`}
                    onClick={onLike}
                >
                    <span className="card-like-icon">♥</span>
                    <span className="card-like-count">{likes}</span>
                </button>

                {/* Phase 3: reveal algorithmic reach */}
                {showReach && (
                    <div className="card-reach" title="algorithmic reach">
                        <span className="card-reach-label">reach</span>
                        <div className="card-reach-bar">
                            <div
                                className="card-reach-fill"
                                style={{
                                    width: `${Math.min(100, (post.reachPct || 0.1) * 100)}%`,
                                    backgroundColor: isDeep ? "#ef4444" : "#22c55e",
                                }}
                            />
                        </div>
                        <span className="card-reach-pct">
              {((post.reachPct || 0.1) * 100).toFixed(0)}%
            </span>
                    </div>
                )}
            </div>

            {/* Phase 2: Corruption overlay on deep content */}
            {phase >= PHASE.CORRUPTION && isDeep && !isHuman && (
                <div className="card-suppress-hint">thuật toán đang giới hạn bài này</div>
            )}
        </article>
    );
}

// Random emoji cho agent dựa trên ID
function agentEmoji(id) {
    const emojis = ["◉", "◈", "◎", "◍", "◌", "◐", "◑", "◒", "◓"];
    const code = id ? id.charCodeAt(0) % emojis.length : 0;
    return emojis[code];
}