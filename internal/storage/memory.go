package storage

import (
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/ai-soft-skills-trainer/internal/models"
)

type Store interface {
	CreateUser(user models.User) error
	GetUser(userID string) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)
	UpdateUser(user models.User) error
	CreateSession(session models.Session) error
	GetSession(sessionID string) (*models.Session, error)
	ListSessionsByUser(userID string) ([]models.SessionSummary, error)
	SaveMessage(sessionID string, message models.Message) error
	SaveFeedback(feedback models.Feedback) error
	GetFeedback(sessionID string) (*models.Feedback, error)
}

type InMemoryStore struct {
	users    map[string]models.User
	emails   map[string]string
	sessions map[string]models.Session
	feedback map[string]models.Feedback
	mu       sync.RWMutex
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		users:    make(map[string]models.User),
		emails:   make(map[string]string),
		sessions: make(map[string]models.Session),
		feedback: make(map[string]models.Feedback),
	}
}

func (m *InMemoryStore) CreateUser(user models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	email := normalizeEmail(user.Email)
	if _, exists := m.emails[email]; exists {
		return errors.New("user with this email already exists")
	}

	user.Email = email
	m.users[user.ID] = user
	m.emails[email] = user.ID
	return nil
}

func (m *InMemoryStore) GetUser(userID string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.users[userID]
	if !ok {
		return nil, errors.New("user not found")
	}
	return &user, nil
}

func (m *InMemoryStore) GetUserByEmail(email string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userID, ok := m.emails[normalizeEmail(email)]
	if !ok {
		return nil, errors.New("user not found")
	}
	user := m.users[userID]
	return &user, nil
}

func (m *InMemoryStore) UpdateUser(user models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.users[user.ID]; !ok {
		return errors.New("user not found")
	}
	user.Email = normalizeEmail(user.Email)
	m.users[user.ID] = user
	m.emails[user.Email] = user.ID
	return nil
}

func (m *InMemoryStore) CreateSession(session models.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.ID] = session
	return nil
}

func (m *InMemoryStore) GetSession(sessionID string) (*models.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, errors.New("session not found")
	}
	return &session, nil
}

func (m *InMemoryStore) ListSessionsByUser(userID string) ([]models.SessionSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]models.SessionSummary, 0)
	for _, session := range m.sessions {
		if session.UserID != userID {
			continue
		}
		items = append(items, models.SessionSummary{
			ID:            session.ID,
			Scenario:      session.Scenario,
			ScenarioTitle: session.ScenarioTitle,
			CreatedAt:     session.CreatedAt,
			UpdatedAt:     session.UpdatedAt,
			MessageCount:  len(session.Messages),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	return items, nil
}

func (m *InMemoryStore) SaveMessage(sessionID string, message models.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return errors.New("session not found")
	}
	session.Messages = append(session.Messages, message)
	session.UpdatedAt = message.CreatedAt
	m.sessions[sessionID] = session
	return nil
}

func (m *InMemoryStore) SaveFeedback(feedback models.Feedback) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.feedback[feedback.SessionID] = feedback
	return nil
}

func (m *InMemoryStore) GetFeedback(sessionID string) (*models.Feedback, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	feedback, ok := m.feedback[sessionID]
	if !ok {
		return nil, errors.New("feedback not found")
	}
	return &feedback, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
