package main

import (
	"context"
	"os"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	mysqldb "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql"
	mysqlrepo "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/telegram"
	notifuc "github.com/ArtemHvozdov/bestieverse.git/internal/usecase/notification"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	tele "gopkg.in/telebot.v3"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level, cfg.Log.File)

	db, err := mysqldb.Open(cfg.DB.DSN())
	if err != nil {
		log.Fatal().Err(err).Msg("db connect")
	}
	defer db.Close()

	bot, err := telegram.NewBot(cfg.Bot.Token, tele.Settings{
		Poller: &tele.LongPoller{Timeout: 10},
	})
	if err != nil {
		log.Fatal().Err(err).Msg("bot init")
	}

	gameRepo := mysqlrepo.NewGameRepo(db)
	notifRepo := mysqlrepo.NewNotificationRepo(db)

	reminderSender := notifuc.NewReminderSender(gameRepo, notifRepo, bot, cfg, log)

	log.Info().Msg("notifier started")

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := reminderSender.SendReminders(context.Background()); err != nil {
			log.Error().Err(err).Msg("notifier: send reminders")
		}
	}
}
