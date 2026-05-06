package main

import (
	"os"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/delivery/bot/handler"
	botmw "github.com/ArtemHvozdov/bestieverse.git/internal/delivery/bot/middleware"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	mysqldb "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql"
	mysqlrepo "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/telegram"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/game"
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

	// Repositories
	gameRepo := mysqlrepo.NewGameRepo(db)
	playerRepo := mysqlrepo.NewPlayerRepo(db)
	playerStateRepo := mysqlrepo.NewPlayerStateRepo(db)

	// Bot
	bot, err := telegram.NewBot(cfg.Bot.Token, tele.Settings{
		Poller: &tele.LongPoller{Timeout: 10},
	})
	if err != nil {
		log.Fatal().Err(err).Msg("bot init")
	}

	// Media
	mediaStorage := media.NewLocalStorage(cfg.Media.Path)

	// Usecases
	creator := game.NewCreator(gameRepo, playerRepo, playerStateRepo, log)
	joiner := game.NewJoiner(gameRepo, playerRepo, playerStateRepo, bot, &cfg.Messages, &cfg.Timings, log)
	leaver := game.NewLeaver(playerRepo, bot, &cfg.Messages, &cfg.Timings, log)
	starter := game.NewStarter(gameRepo, mediaStorage, bot, nil, cfg, log) // publisher wired in Stage 5

	// Handlers
	chatMemberHandler := handler.NewChatMemberHandler(creator, bot, cfg, log)
	callbackHandler := handler.NewCallbackHandler(joiner, leaver, starter, log)

	// Middleware
	pc := botmw.PlayerCheck(gameRepo, playerRepo, bot, &cfg.Messages, &cfg.Timings, log)

	// Global middleware
	bot.Use(botmw.Recover(log))

	// Routes
	bot.Handle(tele.OnMyChatMember, chatMemberHandler.OnMyChatMember)
	bot.Handle("\fgame:join", callbackHandler.OnJoin)
	bot.Handle("\fgame:leave", callbackHandler.OnLeave, pc)
	bot.Handle("\fgame:leave_confirm", callbackHandler.OnLeaveConfirm, pc)
	bot.Handle("\fgame:leave_cancel", callbackHandler.OnLeaveCancel, pc)
	bot.Handle("\fgame:start", callbackHandler.OnStart, pc)

	log.Info().Msg("bot started")
	bot.Start()
}
