package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"

	botpkg "hadith-bot/internal/bot"
	"hadith-bot/internal/config"
	"hadith-bot/internal/image"
	"hadith-bot/internal/logger"
	"hadith-bot/internal/services"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Load .env file if exists
	godotenv.Load()

	// Load configuration
	cfg := config.Load()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Create logger
	logLevel := logger.InfoLevel
	switch cfg.LogLevel {
	case "debug":
		logLevel = logger.DebugLevel
	case "warn":
		logLevel = logger.WarnLevel
	case "error":
		logLevel = logger.ErrorLevel
	}
	log := logger.New(os.Stdout, logLevel, true)
	log.WithPrefix("hadith-bot")

	log.Info("Starting Hadith Portal Bot...")

	// Create Telegram bot using v5 library
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatal("Failed to create bot: %v", err)
	}

	// Set debug mode based on config
	bot.Debug = (cfg.LogLevel == "debug")

	log.Info("Bot created successfully: %s", bot.Self.UserName)

	// Create hadith service
	dataDir := "../../src/data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		dataDir = "../src/data"
	}
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		dataDir = "./data"
	}
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		dataDir = ""
	}

	hadithService := services.NewHadithService(dataDir, cfg.APIURL, cfg.APIKey, cfg.APITimeout, log)
	log.Info("Hadith service initialized")

	// Create image generator
	imageGenerator := image.NewGenerator("./assets/fonts", "./assets/backgrounds")
	log.Info("Image generator initialized")

	// Initialize state manager
	stateManager := botpkg.NewStateManager("./data/state.json")
	log.Info("State manager initialized")

	// Create handler
	handler := botpkg.NewHandler(
		bot,
		hadithService,
		log,
		cfg.RateLimitRequests,
		cfg.RateLimitWindow,
		imageGenerator,
		stateManager,
		cfg.ImageCacheChannelID,
	)

	log.Info("Bot is ready to handle commands")

	// Start scheduler for random hadiths
	handler.StartScheduler()
	log.Info("Scheduler started")

	// Handle graceful shutdown
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		log.Info("Shutting down bot...")
		os.Exit(0)
	}()

	// Start bot update loop (this replaces bot.Start() and handler.HandleCommands())
	handler.StartListening()
}
