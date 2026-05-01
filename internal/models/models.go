package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DailyLimit   int       `json:"daily_limit"`
	UsedToday    int       `json:"used_today"`
	UsageDate    string    `json:"usage_date"`
	CreatedAt    time.Time `json:"created_at"`
}

type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type Session struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Scenario      string    `json:"scenario"`
	ScenarioTitle string    `json:"scenario_title"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Messages      []Message `json:"messages"`
}

type SessionSummary struct {
	ID            string    `json:"id"`
	Scenario      string    `json:"scenario"`
	ScenarioTitle string    `json:"scenario_title"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	MessageCount  int       `json:"message_count"`
}

type Feedback struct {
	SessionID     string    `json:"session_id"`
	Score         int       `json:"score"`
	Strengths     []string  `json:"strengths"`
	Improvements  []string  `json:"improvements"`
	Summary       string    `json:"summary"`
	BetterExample string    `json:"better_example"`
	CreatedAt     time.Time `json:"created_at"`
}
