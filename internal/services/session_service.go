package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ai-soft-skills-trainer/internal/ai"
	"github.com/ai-soft-skills-trainer/internal/models"
	"github.com/ai-soft-skills-trainer/internal/storage"
)

const defaultDailyLimit = 5

type Scenario struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Skill       string   `json:"skill"`
	Difficulty  string   `json:"difficulty"`
	Description string   `json:"description"`
	Goals       []string `json:"goals"`
	Prompt      string   `json:"-"`
}

type SessionService struct {
	store      storage.Store
	aiClient   *ai.Client
	dailyLimit int
}

func NewSessionService(store storage.Store, aiClient *ai.Client, dailyLimit int) *SessionService {
	if dailyLimit <= 0 {
		dailyLimit = defaultDailyLimit
	}
	return &SessionService{store: store, aiClient: aiClient, dailyLimit: dailyLimit}
}

func (s *SessionService) Register(name, email, password string) (*models.User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" || email == "" || len(password) < 6 {
		return nil, errors.New("name, valid email and password with at least 6 characters are required")
	}

	now := time.Now().UTC()
	user := models.User{
		ID:           uuid.NewString(),
		Name:         name,
		Email:        email,
		PasswordHash: hashPassword(password),
		DailyLimit:   s.dailyLimit,
		UsedToday:    0,
		UsageDate:    todayKey(),
		CreatedAt:    now,
	}
	if err := s.store.CreateUser(user); err != nil {
		return nil, err
	}
	return sanitizeUser(user), nil
}

func (s *SessionService) Login(email, password string) (*models.User, error) {
	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}
	if user.PasswordHash != hashPassword(password) {
		return nil, errors.New("invalid email or password")
	}
	if err := s.resetUsageIfNeeded(user); err != nil {
		return nil, err
	}
	return sanitizeUser(*user), nil
}

func (s *SessionService) Profile(userID string) (*models.User, []models.SessionSummary, error) {
	user, err := s.store.GetUser(userID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.resetUsageIfNeeded(user); err != nil {
		return nil, nil, err
	}
	sessions, err := s.store.ListSessionsByUser(userID)
	if err != nil {
		return nil, nil, err
	}
	return sanitizeUser(*user), sessions, nil
}

func (s *SessionService) ListScenarios() []Scenario {
	items := make([]Scenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		items = append(items, publicScenario(scenario))
	}
	return items
}

func (s *SessionService) StartSession(ctx context.Context, userID, scenarioID string) (*models.Session, Scenario, string, error) {
	user, err := s.store.GetUser(userID)
	if err != nil {
		return nil, Scenario{}, "", err
	}
	if err := s.consumeDailyUse(user); err != nil {
		return nil, Scenario{}, "", err
	}

	scenario, err := pickScenario(scenarioID)
	if err != nil {
		return nil, Scenario{}, "", err
	}
	scenario = randomizeScenario(scenario)

	now := time.Now().UTC()
	id := uuid.NewString()
	session := models.Session{
		ID:            id,
		UserID:        userID,
		Scenario:      scenario.ID,
		ScenarioTitle: scenario.Title,
		CreatedAt:     now,
		UpdatedAt:     now,
		Messages:      []models.Message{},
	}

	assistantText, err := s.aiClient.SendMessage(ctx, buildRoleplayPrompt(scenario, true), []models.Message{})
	if err != nil {
		user.UsedToday--
		_ = s.store.UpdateUser(*user)
		return nil, Scenario{}, "", err
	}

	assistantMessage := models.Message{
		ID:        uuid.NewString(),
		SessionID: id,
		Role:      "assistant",
		Text:      assistantText,
		CreatedAt: time.Now().UTC(),
	}
	session.Messages = append(session.Messages, assistantMessage)
	session.UpdatedAt = assistantMessage.CreatedAt

	if err := s.store.CreateSession(session); err != nil {
		return nil, Scenario{}, "", err
	}

	return &session, publicScenario(scenario), assistantText, nil
}

func (s *SessionService) SendMessage(ctx context.Context, userID, sessionID, userText string) (string, error) {
	session, err := s.store.GetSession(sessionID)
	if err != nil {
		return "", err
	}
	if session.UserID != userID {
		return "", errors.New("session does not belong to user")
	}

	scenario, err := findScenario(session.Scenario)
	if err != nil {
		return "", err
	}

	userMessage := models.Message{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Role:      "user",
		Text:      userText,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.SaveMessage(sessionID, userMessage); err != nil {
		return "", err
	}

	response, err := s.aiClient.SendMessage(ctx, buildRoleplayPrompt(scenario, false), append(session.Messages, userMessage))
	if err != nil {
		return "", err
	}

	assistantMessage := models.Message{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Role:      "assistant",
		Text:      response,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.SaveMessage(sessionID, assistantMessage); err != nil {
		return "", err
	}

	return response, nil
}

func (s *SessionService) GetSession(userID, sessionID string) (*models.Session, error) {
	session, err := s.store.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, errors.New("session does not belong to user")
	}
	return session, nil
}

func (s *SessionService) GetFeedback(ctx context.Context, userID, sessionID string) (*models.Feedback, error) {
	session, err := s.GetSession(userID, sessionID)
	if err != nil {
		return nil, err
	}
	if len(session.Messages) == 0 {
		return nil, errors.New("no messages to analyze")
	}

	scenario, err := findScenario(session.Scenario)
	if err != nil {
		return nil, err
	}

	response, err := s.aiClient.SendMessage(ctx, buildFeedbackPrompt(scenario), session.Messages)
	if err != nil {
		return nil, err
	}

	feedback, err := parseFeedback(response)
	if err != nil {
		return nil, err
	}

	feedback.SessionID = sessionID
	feedback.CreatedAt = time.Now().UTC()
	if err := s.store.SaveFeedback(*feedback); err != nil {
		return nil, err
	}

	return feedback, nil
}

func (s *SessionService) GetModels(ctx context.Context) ([]byte, error) {
	return s.aiClient.GetModels(ctx)
}

func (s *SessionService) consumeDailyUse(user *models.User) error {
	if err := s.resetUsageIfNeeded(user); err != nil {
		return err
	}
	if user.UsedToday >= user.DailyLimit {
		return errors.New("daily training limit reached")
	}
	user.UsedToday++
	return s.store.UpdateUser(*user)
}

func (s *SessionService) resetUsageIfNeeded(user *models.User) error {
	today := todayKey()
	if user.UsageDate == today {
		return nil
	}
	user.UsageDate = today
	user.UsedToday = 0
	return s.store.UpdateUser(*user)
}

func pickScenario(id string) (Scenario, error) {
	if strings.TrimSpace(id) == "" || id == "random" {
		return scenarios[randomInt(len(scenarios))], nil
	}
	return findScenario(id)
}

func findScenario(id string) (Scenario, error) {
	for _, scenario := range scenarios {
		if scenario.ID == id || scenario.Title == id {
			return scenario, nil
		}
	}
	return Scenario{}, errors.New("unknown scenario")
}

func randomizeScenario(scenario Scenario) Scenario {
	context := randomContext()
	scenario.Description = scenario.Description + " " + context
	scenario.Prompt = scenario.Prompt + "\nДополнительный контекст этой попытки: " + context
	return scenario
}

func randomContext() string {
	contexts := []string{
		"Собеседник спешит на следующую встречу и быстро теряет терпение.",
		"Перед разговором уже была одна неудачная попытка договориться.",
		"В разговоре есть скрытый риск: собеседник боится потерять контроль над ситуацией.",
		"Собеседник сначала отвечает коротко и проверяет, будете ли вы задавать уточняющие вопросы.",
		"Ситуация происходит в конце напряженной рабочей недели.",
	}
	return contexts[randomInt(len(contexts))]
}

func randomInt(limit int) int {
	return rand.New(rand.NewSource(time.Now().UnixNano())).Intn(limit)
}

func buildRoleplayPrompt(scenario Scenario, initial bool) string {
	mode := "continue"
	if initial {
		mode = "start"
	}

	return `Ты генеративный тренажер soft skills. Работай как живой собеседник в ролевой сцене, а не как помощник, который объясняет правила.

MODE: roleplay
STEP: ` + mode + `
SCENARIO: ` + scenario.Title + `
SKILL: ` + scenario.Skill + `
DIFFICULTY: ` + scenario.Difficulty + `
ROLE:
` + scenario.Prompt + `

TRAINING GOALS:
- ` + strings.Join(scenario.Goals, "\n- ") + `

STRICT RULES:
- Отвечай только на русском.
- Не говори, что ты модель, ассистент или тренажер.
- Не давай советов пользователю внутри сцены и не оценивай его.
- Говори от лица роли: естественно, с эмоцией и конкретными претензиями или вопросами.
- Избегай банальных фраз вроде "давайте обсудим" без конкретного давления.
- Каждая реплика должна создавать новый поворот: уточнение, возражение, сомнение, давление, уступка или проверка договоренности.
- Длина ответа: 1-3 коротких предложения.
- Если это первая реплика, сразу начни конфликт или рабочий вопрос.`
}

func buildFeedbackPrompt(scenario Scenario) string {
	return `Ты эксперт по soft skills и оцениваешь завершенный тренировочный диалог.

MODE: feedback
SCENARIO: ` + scenario.Title + `
SKILL: ` + scenario.Skill + `
DIFFICULTY: ` + scenario.Difficulty + `

Отвечай только валидным JSON-объектом. Никаких markdown-блоков, списка, пояснений или текста перед/после JSON.
{
  "score": number from 0 to 10,
  "strengths": [string],
  "improvements": [string],
  "summary": string,
  "better_example": string
}

Оценивай только реплики пользователя. Критерии: эмпатия, ясность, активное слушание, границы, конкретный следующий шаг, спокойствие под давлением. Пиши конкретно, без общих похвал.`
}

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}

func parseFeedback(raw string) (*models.Feedback, error) {
	jsonText := extractJSON(raw)
	if jsonText == "" {
		return nil, errors.New("feedback response is not valid json")
	}

	var feedback models.Feedback
	if err := json.Unmarshal([]byte(jsonText), &feedback); err != nil {
		return nil, err
	}

	if feedback.Score < 0 {
		feedback.Score = 0
	}
	if feedback.Score > 10 {
		feedback.Score = 10
	}
	return &feedback, nil
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte("soft-skills-trainer:" + password))
	return hex.EncodeToString(sum[:])
}

func sanitizeUser(user models.User) *models.User {
	user.PasswordHash = ""
	return &user
}

func todayKey() string {
	return time.Now().UTC().Format("2006-01-02")
}

func publicScenario(scenario Scenario) Scenario {
	scenario.Prompt = ""
	return scenario
}

var scenarios = []Scenario{
	{
		ID:          "conflict-colleague",
		Title:       "Коллега сопротивляется идее",
		Skill:       "Аргументация и деэскалация",
		Difficulty:  "Средний",
		Description: "Коллега резко критикует вашу инициативу на рабочей встрече. Нужно удержать контакт и перевести спор в маленький проверяемый шаг.",
		Goals: []string{
			"признать эмоцию без капитуляции",
			"сформулировать ценность идеи простыми словами",
			"договориться о следующем шаге",
		},
		Prompt: "Ты опытный, но уставший коллега. Ты сомневаешься в новой идее, боишься лишней работы и просишь конкретики. Тебя можно убедить, если пользователь признает риски и предложит небольшой эксперимент.",
	},
	{
		ID:          "angry-client",
		Title:       "Недовольный клиент",
		Skill:       "Эмпатия и восстановление доверия",
		Difficulty:  "Сложный",
		Description: "Клиент злится из-за задержки результата. Нужно услышать претензию, взять ответственность и не обещать невозможного.",
		Goals: []string{
			"снизить эмоциональное напряжение",
			"выяснить главный ущерб для клиента",
			"дать прозрачный план исправления",
		},
		Prompt: "Ты клиент, который уже дважды получал обещания и больше не верит команде. Ты говоришь резко, требуешь сроков и проверяешь, не прячется ли пользователь за общими фразами.",
	},
	{
		ID:          "manager-feedback",
		Title:       "Обратная связь сотруднику",
		Skill:       "Честность без давления",
		Difficulty:  "Средний",
		Description: "Сотрудник пропустил несколько сроков и защищается. Нужно назвать проблему, сохранить уважение и договориться о поддержке.",
		Goals: []string{
			"говорить о фактах, а не ярлыках",
			"проверить причины срывов",
			"закрепить договоренность и критерий успеха",
		},
		Prompt: "Ты сотрудник, который чувствует несправедливость и боится, что его считают слабым. Ты защищаешься, но готов открыться, если пользователь говорит спокойно и конкретно.",
	},
	{
		ID:          "interview-leadership",
		Title:       "Интервью на лидерскую роль",
		Skill:       "Самопрезентация и STAR-ответы",
		Difficulty:  "Базовый",
		Description: "Интервьюер просит примеры лидерства, конфликтов и решений под давлением. Нужно отвечать структурно и без воды.",
		Goals: []string{
			"дать контекст, действие и результат",
			"показать ответственность за решение",
			"избежать расплывчатых формулировок",
		},
		Prompt: "Ты интервьюер на лидерскую позицию. Ты задаешь точные вопросы, просишь примеры и мягко прерываешь, если ответ слишком общий.",
	},
	{
		ID:          "salary-talk",
		Title:       "Разговор о повышении",
		Skill:       "Уверенность и переговоры",
		Difficulty:  "Сложный",
		Description: "Руководитель сомневается, что повышение сейчас обосновано. Нужно говорить о ценности, фактах и вариантах без давления.",
		Goals: []string{
			"назвать измеримый вклад",
			"спокойно выдержать возражение",
			"предложить понятный следующий шаг",
		},
		Prompt: "Ты руководитель, который не хочет сразу соглашаться на повышение. Ты задаешь вопросы про результаты, бюджет и ожидания. Ты готов обсуждать план роста, если пользователь говорит фактами.",
	},
	{
		ID:          "cross-team-alignment",
		Title:       "Согласование между командами",
		Skill:       "Фасилитация и ясные договоренности",
		Difficulty:  "Средний",
		Description: "Две команды спорят о приоритетах и ответственности. Нужно снять взаимные обвинения и зафиксировать рабочую договоренность.",
		Goals: []string{
			"разделить факты и интерпретации",
			"согласовать владельца следующего шага",
			"оставить письменный критерий успеха",
		},
		Prompt: "Ты представитель соседней команды. Ты раздражен, потому что считаешь, что вашу команду постоянно ставят перед фактом. Ты готов договариваться, если пользователь признает влияние на вашу работу.",
	},
}
