package storage

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ai-soft-skills-trainer/internal/models"
)

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	if err := initSchema(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &PostgresStore{db: pool}, nil
}

func initSchema(ctx context.Context, db *pgxpool.Pool) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    daily_limit INT NOT NULL,
    used_today INT NOT NULL,
    usage_date TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scenario TEXT NOT NULL,
    scenario_title TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS feedback (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    score INT NOT NULL,
    strengths JSONB NOT NULL,
    improvements JSONB NOT NULL,
    summary TEXT NOT NULL,
    better_example TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
`
	_, err := db.Exec(ctx, schema)
	return err
}

func (p *PostgresStore) CreateUser(user models.User) error {
	ctx := context.Background()
	_, err := p.db.Exec(ctx, `
INSERT INTO users (id, name, email, password_hash, daily_limit, used_today, usage_date, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`, user.ID, user.Name, normalizeEmail(user.Email), user.PasswordHash, user.DailyLimit, user.UsedToday, user.UsageDate, user.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			return errors.New("user with this email already exists")
		}
		return err
	}
	return nil
}

func (p *PostgresStore) GetUser(userID string) (*models.User, error) {
	ctx := context.Background()
	row := p.db.QueryRow(ctx, `
SELECT id, name, email, password_hash, daily_limit, used_today, usage_date, created_at
FROM users WHERE id = $1
`, userID)

	var user models.User
	if err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.DailyLimit, &user.UsedToday, &user.UsageDate, &user.CreatedAt); err != nil {
		return nil, errors.New("user not found")
	}
	return &user, nil
}

func (p *PostgresStore) GetUserByEmail(email string) (*models.User, error) {
	ctx := context.Background()
	row := p.db.QueryRow(ctx, `
SELECT id, name, email, password_hash, daily_limit, used_today, usage_date, created_at
FROM users WHERE email = $1
`, normalizeEmail(email))

	var user models.User
	if err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.DailyLimit, &user.UsedToday, &user.UsageDate, &user.CreatedAt); err != nil {
		return nil, errors.New("user not found")
	}
	return &user, nil
}

func (p *PostgresStore) UpdateUser(user models.User) error {
	ctx := context.Background()
	cmd, err := p.db.Exec(ctx, `
UPDATE users
SET name = $1, email = $2, password_hash = $3, daily_limit = $4, used_today = $5, usage_date = $6
WHERE id = $7
`, user.Name, normalizeEmail(user.Email), user.PasswordHash, user.DailyLimit, user.UsedToday, user.UsageDate, user.ID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (p *PostgresStore) CreateSession(session models.Session) error {
	ctx := context.Background()
	_, err := p.db.Exec(ctx, `
INSERT INTO sessions (id, user_id, scenario, scenario_title, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
`, session.ID, session.UserID, session.Scenario, session.ScenarioTitle, session.CreatedAt, session.UpdatedAt)
	return err
}

func (p *PostgresStore) GetSession(sessionID string) (*models.Session, error) {
	ctx := context.Background()
	row := p.db.QueryRow(ctx, `
SELECT id, user_id, scenario, scenario_title, created_at, updated_at
FROM sessions WHERE id = $1
`, sessionID)

	var session models.Session
	if err := row.Scan(&session.ID, &session.UserID, &session.Scenario, &session.ScenarioTitle, &session.CreatedAt, &session.UpdatedAt); err != nil {
		return nil, errors.New("session not found")
	}

	rows, err := p.db.Query(ctx, `
SELECT id, role, text, created_at FROM messages
WHERE session_id = $1 ORDER BY created_at ASC
`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.Role, &msg.Text, &msg.CreatedAt); err != nil {
			return nil, err
		}
		msg.SessionID = sessionID
		session.Messages = append(session.Messages, msg)
	}

	return &session, nil
}

func (p *PostgresStore) ListSessionsByUser(userID string) ([]models.SessionSummary, error) {
	ctx := context.Background()
	rows, err := p.db.Query(ctx, `
SELECT s.id, s.scenario, s.scenario_title, s.created_at, s.updated_at,
       COALESCE(m.message_count, 0) AS message_count
FROM sessions s
LEFT JOIN (
    SELECT session_id, COUNT(*) AS message_count FROM messages GROUP BY session_id
) m ON m.session_id = s.id
WHERE s.user_id = $1
ORDER BY s.updated_at DESC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.SessionSummary
	for rows.Next() {
		var item models.SessionSummary
		if err := rows.Scan(&item.ID, &item.Scenario, &item.ScenarioTitle, &item.CreatedAt, &item.UpdatedAt, &item.MessageCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (p *PostgresStore) SaveMessage(sessionID string, message models.Message) error {
	ctx := context.Background()
	cmd, err := p.db.Exec(ctx, `
INSERT INTO messages (id, session_id, role, text, created_at)
VALUES ($1, $2, $3, $4, $5)
`, message.ID, sessionID, message.Role, message.Text, message.CreatedAt)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("message was not saved")
	}

	_, err = p.db.Exec(ctx, `
UPDATE sessions SET updated_at = $1 WHERE id = $2
`, message.CreatedAt, sessionID)
	return err
}

func (p *PostgresStore) SaveFeedback(feedback models.Feedback) error {
	ctx := context.Background()
	strengthsJSON, err := json.Marshal(feedback.Strengths)
	if err != nil {
		return err
	}
	improvementsJSON, err := json.Marshal(feedback.Improvements)
	if err != nil {
		return err
	}
	_, err = p.db.Exec(ctx, `
INSERT INTO feedback (session_id, score, strengths, improvements, summary, better_example, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (session_id) DO UPDATE SET
    score = EXCLUDED.score,
    strengths = EXCLUDED.strengths,
    improvements = EXCLUDED.improvements,
    summary = EXCLUDED.summary,
    better_example = EXCLUDED.better_example,
    created_at = EXCLUDED.created_at
`, feedback.SessionID, feedback.Score, strengthsJSON, improvementsJSON, feedback.Summary, feedback.BetterExample, feedback.CreatedAt)
	return err
}

func (p *PostgresStore) GetFeedback(sessionID string) (*models.Feedback, error) {
	ctx := context.Background()
	row := p.db.QueryRow(ctx, `
SELECT session_id, score, strengths, improvements, summary, better_example, created_at
FROM feedback WHERE session_id = $1
`, sessionID)

	var feedback models.Feedback
	var strengthsData, improvementsData []byte
	if err := row.Scan(&feedback.SessionID, &feedback.Score, &strengthsData, &improvementsData, &feedback.Summary, &feedback.BetterExample, &feedback.CreatedAt); err != nil {
		return nil, errors.New("feedback not found")
	}
	if err := json.Unmarshal(strengthsData, &feedback.Strengths); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(improvementsData, &feedback.Improvements); err != nil {
		return nil, err
	}
	return &feedback, nil
}

func (p *PostgresStore) GetUserFeedbackStats(userID string) (models.UserFeedbackStats, error) {
	ctx := context.Background()
	stats := models.UserFeedbackStats{Trend: "stable"}

	if err := p.db.QueryRow(ctx, `
SELECT COUNT(*) FROM sessions WHERE user_id = $1
`, userID).Scan(&stats.CompletedSessions); err != nil {
		return stats, err
	}

	rows, err := p.db.Query(ctx, `
SELECT f.score
FROM feedback f
JOIN sessions s ON s.id = f.session_id
WHERE s.user_id = $1
ORDER BY f.created_at DESC
`, userID)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	scores := make([]int, 0)
	for rows.Next() {
		var score int
		if err := rows.Scan(&score); err != nil {
			return stats, err
		}
		scores = append(scores, score)
		if len(stats.RecentScores) < 12 {
			stats.RecentScores = append(stats.RecentScores, score)
		}
	}

	stats.FeedbackCount = len(scores)
	if stats.FeedbackCount == 0 {
		return stats, nil
	}

	total := 0
	for _, score := range scores {
		total += score
	}
	stats.AverageScore = float64(total) / float64(stats.FeedbackCount)
	stats.AveragePercent = int(math.Round(stats.AverageScore * 10))

	if len(scores) >= 2 {
		split := len(scores) / 2
		recent := averageIntScores(scores[:split])
		previous := averageIntScores(scores[split:])
		stats.RecentAverage = recent
		stats.PreviousAverage = previous
		if recent-previous >= 0.5 {
			stats.Trend = "up"
		} else if previous-recent >= 0.5 {
			stats.Trend = "down"
		}
	} else {
		stats.RecentAverage = float64(scores[0])
		stats.PreviousAverage = float64(scores[0])
	}

	return stats, nil
}
