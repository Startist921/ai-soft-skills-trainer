package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ai-soft-skills-trainer/internal/models"
	"github.com/ai-soft-skills-trainer/internal/services"
)

type Handler struct {
	sessionService *services.SessionService
}

func NewHandler(sessionService *services.SessionService) *Handler {
	return &Handler{sessionService: sessionService}
}

type authRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	User *models.User `json:"user"`
}

type profileResponse struct {
	User     *models.User            `json:"user"`
	Sessions []models.SessionSummary `json:"sessions"`
}

type startSessionRequest struct {
	Scenario string `json:"scenario"`
}

type startSessionResponse struct {
	SessionID      string            `json:"session_id"`
	Scenario       string            `json:"scenario"`
	ScenarioData   services.Scenario `json:"scenario_data"`
	InitialMessage string            `json:"initial_message,omitempty"`
}

type sendMessageRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type sendMessageResponse struct {
	SessionID string `json:"session_id"`
	Reply     string `json:"reply"`
}

type feedbackResponse struct {
	Score         int      `json:"score"`
	Strengths     []string `json:"strengths"`
	Improvements  []string `json:"improvements"`
	Summary       string   `json:"summary"`
	BetterExample string   `json:"better_example"`
}

func (h *Handler) Register(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user, err := h.sessionService.Register(req.Name, req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, authResponse{User: user})
}

func (h *Handler) Login(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user, err := h.sessionService.Login(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, authResponse{User: user})
}

func (h *Handler) Profile(c *gin.Context) {
	userID, ok := h.requireUser(c)
	if !ok {
		return
	}

	user, sessions, err := h.sessionService.Profile(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, profileResponse{User: user, Sessions: sessions})
}

func (h *Handler) ListScenarios(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"scenarios": h.sessionService.ListScenarios()})
}

func (h *Handler) StartSession(c *gin.Context) {
	userID, ok := h.requireUser(c)
	if !ok {
		return
	}

	var req startSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Scenario == "" {
		req.Scenario = "random"
	}

	session, scenario, initialMessage, err := h.sessionService.StartSession(c.Request.Context(), userID, req.Scenario)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "daily training limit reached" {
			status = http.StatusTooManyRequests
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, startSessionResponse{
		SessionID:      session.ID,
		Scenario:       session.Scenario,
		ScenarioData:   scenario,
		InitialMessage: initialMessage,
	})
}

func (h *Handler) SendMessage(c *gin.Context) {
	userID, ok := h.requireUser(c)
	if !ok {
		return
	}

	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id and message are required"})
		return
	}

	reply, err := h.sessionService.SendMessage(c.Request.Context(), userID, req.SessionID, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sendMessageResponse{SessionID: req.SessionID, Reply: reply})
}

func (h *Handler) GetSession(c *gin.Context) {
	userID, ok := h.requireUser(c)
	if !ok {
		return
	}

	sessionID := c.Param("id")
	session, err := h.sessionService.GetSession(userID, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (h *Handler) GetFeedback(c *gin.Context) {
	userID, ok := h.requireUser(c)
	if !ok {
		return
	}

	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	feedback, err := h.sessionService.GetFeedback(c.Request.Context(), userID, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, feedbackResponse{
		Score:         feedback.Score,
		Strengths:     feedback.Strengths,
		Improvements:  feedback.Improvements,
		Summary:       feedback.Summary,
		BetterExample: feedback.BetterExample,
	})
}

func (h *Handler) CheckModels(c *gin.Context) {
	modelsBytes, err := h.sessionService.GetModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", modelsBytes)
}

func (h *Handler) requireUser(c *gin.Context) (string, bool) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-User-ID header is required"})
		return "", false
	}
	return userID, true
}
