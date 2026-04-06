-- Tạo database (bỏ qua nếu đã có)
IF NOT EXISTS (SELECT name FROM sys.databases WHERE name = 'validation_sim')
    CREATE DATABASE validation_sim;
GO

USE validation_sim;
GO

-- Bảng Users — thực thể nghiên cứu chính
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'users')
CREATE TABLE users (
                       id                    UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
                       personality_type      VARCHAR(10)  NOT NULL
                           CHECK (personality_type IN ('Introvert', 'Extrovert', 'Seeker', 'Human')),

    -- Agent logic state [BVL / ET]
                       base_validation_level FLOAT        NOT NULL,
                       engagement_threshold  FLOAT        NOT NULL,
                       inflation_factor      FLOAT        NOT NULL DEFAULT 0, -- Dopamine Inflation tracking

    -- Tích lũy sau mỗi session
                       validation_score      FLOAT        NOT NULL DEFAULT 0,
                       total_active_time     FLOAT        NOT NULL DEFAULT 0, -- seconds
                       post_count            INT          NOT NULL DEFAULT 0,
                       low_effort_ratio      FLOAT        NOT NULL DEFAULT 0, -- CQDI contribution

                       is_human              BIT          NOT NULL DEFAULT 0,
                       created_at            DATETIMEOFFSET NOT NULL DEFAULT SYSDATETIMEOFFSET()
);
GO

-- Bảng simulation_sessions — mỗi lần bấm "run sim"
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'simulation_sessions')
CREATE TABLE simulation_sessions (
                                     id              INT IDENTITY(1,1) PRIMARY KEY,
                                     run_at          DATETIMEOFFSET NOT NULL DEFAULT SYSDATETIMEOFFSET(),
                                     num_agents      INT            NOT NULL,
                                     duration_sec    FLOAT          NOT NULL,

    -- Scenario A (linear)
                                     a_mean_retention    FLOAT NOT NULL DEFAULT 0,
                                     a_mean_cqdi         FLOAT NOT NULL DEFAULT 0,
                                     a_mean_valid_score  FLOAT NOT NULL DEFAULT 0,
                                     a_disengagement_pct FLOAT NOT NULL DEFAULT 0,

    -- Scenario B (variable-ratio)
                                     b_mean_retention    FLOAT NOT NULL DEFAULT 0,
                                     b_mean_cqdi         FLOAT NOT NULL DEFAULT 0,
                                     b_mean_valid_score  FLOAT NOT NULL DEFAULT 0,
                                     b_disengagement_pct FLOAT NOT NULL DEFAULT 0,

    -- Delta — cốt lõi của H1/H2
                                     delta_retention     FLOAT NOT NULL DEFAULT 0,  -- B - A
                                     delta_cqdi          FLOAT NOT NULL DEFAULT 0,

    -- Breakdown theo personality (JSON string vì SQL Server 2016+)
                                     retention_by_personality_json  NVARCHAR(MAX) NULL,
                                     validation_by_personality_json NVARCHAR(MAX) NULL
);
GO

-- Bảng posts — hành động đăng bài của agent
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'posts')
CREATE TABLE posts (
                       id               UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
                       session_id       INT            REFERENCES simulation_sessions(id),
                       user_id          UNIQUEIDENTIFIER REFERENCES users(id),
                       content_type     VARCHAR(5)     NOT NULL CHECK (content_type IN ('Deep', 'Trend')),

    -- Trạng thái nội tâm lúc đăng bài
                       dopamine_at_post FLOAT          NOT NULL,
                       threshold_at_post FLOAT         NOT NULL DEFAULT 0, -- ET tại thời điểm đó

    -- Content Ranker output
                       reach_multiplier FLOAT          NOT NULL DEFAULT 1.0,
                       is_echo_chamber  BIT            NOT NULL DEFAULT 0,

                       created_at       DATETIMEOFFSET NOT NULL DEFAULT SYSDATETIMEOFFSET()
);
GO

-- Bảng interactions — like/reward events
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'interactions')
CREATE TABLE interactions (
                              id              UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
                              session_id      INT              REFERENCES simulation_sessions(id),
                              actor_id        UNIQUEIDENTIFIER REFERENCES users(id),
                              post_id         UNIQUEIDENTIFIER REFERENCES posts(id),

    -- Biến đo lường sự công nhận
                              latency         FLOAT          NOT NULL DEFAULT 0, -- ms từ post đến reward
                              points_awarded  FLOAT          NOT NULL DEFAULT 0,
                              is_burst_reward BIT            NOT NULL DEFAULT 0, -- Scenario B burst flag
                              is_notification BIT            NOT NULL DEFAULT 0, -- top-10% notification

                              created_at      DATETIMEOFFSET NOT NULL DEFAULT SYSDATETIMEOFFSET()
);
GO


-- Bảng simulation_metrics — snapshot 200ms
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'simulation_metrics')
CREATE TABLE simulation_metrics (
                                    id                       INT IDENTITY(1,1) PRIMARY KEY,
                                    session_id               INT              REFERENCES simulation_sessions(id),
                                    agent_id                 UNIQUEIDENTIFIER REFERENCES users(id),
                                    personality              VARCHAR(10)      NOT NULL,
    [timestamp]              DATETIMEOFFSET   NOT NULL DEFAULT SYSDATETIMEOFFSET(),

    current_dopamine         FLOAT            NOT NULL,
    engagement_threshold     FLOAT            NOT NULL,
    inflation_factor         FLOAT            NOT NULL DEFAULT 0,
    validation_score         FLOAT            NOT NULL,
    low_effort_ratio         FLOAT            NOT NULL DEFAULT 0,
    is_disengaged            BIT              NOT NULL DEFAULT 0,
    is_in_top_10             BIT              NOT NULL DEFAULT 0
    );
GO


-- Bảng human_sessions — lịch sử human agent riêng
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'human_sessions')
CREATE TABLE human_sessions (
                                id                 INT IDENTITY(1,1) PRIMARY KEY,
                                session_id         INT              REFERENCES simulation_sessions(id),
                                run_at             DATETIMEOFFSET   NOT NULL DEFAULT SYSDATETIMEOFFSET(),

                                final_dopamine     FLOAT            NOT NULL,
                                final_score        FLOAT            NOT NULL,
                                inflation_factor   FLOAT            NOT NULL DEFAULT 0,
                                active_time_sec    FLOAT            NOT NULL,
                                post_count         INT              NOT NULL DEFAULT 0,
                                low_effort_ratio   FLOAT            NOT NULL DEFAULT 0,

    -- Blind Experience tracking
                                reached_phase2     BIT              NOT NULL DEFAULT 0,
                                reached_phase3     BIT              NOT NULL DEFAULT 0,
                                phase2_at_pct      FLOAT            NULL, -- % active_time khi vào phase 2
                                phase3_at_pct      FLOAT            NULL
);
GO

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'idx_user_validation_score')
CREATE INDEX idx_user_validation_score   ON users(validation_score DESC);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'idx_sessions_run_at')
CREATE INDEX idx_sessions_run_at         ON simulation_sessions(run_at DESC);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'idx_metrics_session')
CREATE INDEX idx_metrics_session         ON simulation_metrics(session_id, [timestamp]);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'idx_posts_session')
CREATE INDEX idx_posts_session           ON posts(session_id);

IF NOT EXISTS (SELECT * FROM sys.indexes WHERE name = 'idx_interactions_post')
CREATE INDEX idx_interactions_post       ON interactions(post_id);
GO

PRINT 'Schema created successfully on validation_sim database.';
GO