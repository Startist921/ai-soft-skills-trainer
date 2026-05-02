package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ai-soft-skills-trainer/internal/ai"
	"github.com/ai-soft-skills-trainer/internal/handlers"
	"github.com/ai-soft-skills-trainer/internal/services"
	"github.com/ai-soft-skills-trainer/internal/storage"
)

func main() {
	loadEnvFile(".env")

	mlServiceURL := os.Getenv("ML_SERVICE_URL")
	if mlServiceURL == "" {
		mlServiceURL = "http://localhost:8087"
	}

	modelName := os.Getenv("MODEL_NAME")
	if modelName == "" {
		modelName = "GigaChat"
	}

	maxNewTokens := 120
	if value := os.Getenv("MAX_NEW_TOKENS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			log.Fatalf("invalid MAX_NEW_TOKENS value: %v", err)
		}
		maxNewTokens = parsed
	}

	temperature := 0.55
	if value := os.Getenv("TEMPERATURE"); value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			log.Fatalf("invalid TEMPERATURE value: %v", err)
		}
		temperature = parsed
	}

	topP := 0.85
	if value := os.Getenv("TOP_P"); value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			log.Fatalf("invalid TOP_P value: %v", err)
		}
		topP = parsed
	}

	dailyLimit := 5
	if value := os.Getenv("DAILY_TRAINING_LIMIT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			log.Fatalf("invalid DAILY_TRAINING_LIMIT value: %v", err)
		}
		dailyLimit = parsed
	}

	store := storage.NewInMemoryStore()
	aiclient := ai.NewClient(mlServiceURL, modelName, maxNewTokens, temperature, topP)
	service := services.NewSessionService(store, aiclient, dailyLimit)
	h := handlers.NewHandler(service)

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := router.Group("/api")
	api.POST("/register", h.Register)
	api.POST("/login", h.Login)
	api.GET("/profile", h.Profile)
	api.GET("/scenarios", h.ListScenarios)
	api.POST("/start-session", h.StartSession)
	api.POST("/send-message", h.SendMessage)
	api.GET("/sessions/:id", h.GetSession)
	api.GET("/get-feedback", h.GetFeedback)
	api.GET("/check-models", h.CheckModels)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting backend at :%s\n", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
