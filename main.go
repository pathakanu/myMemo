package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pathakanu/myMemo/internal/bot"
	"github.com/pathakanu/myMemo/internal/config"
	"github.com/pathakanu/myMemo/internal/database"
	myopenai "github.com/pathakanu/myMemo/internal/openai"
	"github.com/pathakanu/myMemo/internal/twilio"
)

func main() {
	logger := log.New(os.Stdout, "[myMemo] ", log.LstdFlags|log.Lshortfile)
	cfg := config.Load()
	// fmt.Println("Configuration loaded: ", cfg)

	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("database init failed: %v", err)
	}

	openAIClient := myopenai.New(cfg.OpenAIAPIKey)
	fmt.Println("Twilio WhatsApp Number:", cfg.TwilioWhatsAppNumber)
	twilioClient := twilio.New(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioWhatsAppNumber)

	reminderBot := bot.New(cfg, db, openAIClient, twilioClient, logger)
	if err := reminderBot.StartScheduler(); err != nil {
		logger.Fatalf("scheduler start: %v", err)
	}

	http.Handle("/twilio/webhook", reminderBot.Handler())

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: nil,
	}

	go func() {
		logger.Printf("server starting on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	waitForShutdown(server, reminderBot, logger)
}

func waitForShutdown(server *http.Server, reminderBot *bot.Bot, logger *log.Logger) {
	stopCtx := make(chan os.Signal, 1)
	signal.Notify(stopCtx, syscall.SIGINT, syscall.SIGTERM)
	<-stopCtx
	logger.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Printf("server shutdown error: %v", err)
	}
	reminderBot.StopScheduler()
}
