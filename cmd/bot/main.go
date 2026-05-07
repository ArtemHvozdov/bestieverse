package main

import (
	"os"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/delivery/bot/handler"
	botmw "github.com/ArtemHvozdov/bestieverse.git/internal/delivery/bot/middleware"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	mysqldb "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql"
	mysqlrepo "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/openai"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/telegram"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/game"
	taskuc "github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/subtask"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/lock"
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
	taskResponseRepo := mysqlrepo.NewTaskResponseRepo(db)
	taskResultRepo := mysqlrepo.NewTaskResultRepo(db)
	taskLockRepo := mysqlrepo.NewTaskLockRepo(db)
	subtaskProgressRepo := mysqlrepo.NewSubtaskProgressRepo(db)

	// Bot
	bot, err := telegram.NewBot(cfg.Bot.Token, tele.Settings{
		Poller: &tele.LongPoller{Timeout: 10},
	})
	if err != nil {
		log.Fatal().Err(err).Msg("bot init")
	}

	// Media
	mediaStorage := media.NewLocalStorage(cfg.Media.Path)

	// OpenAI client
	openaiClient := openai.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.Model)

	// Lock manager
	lockManager := lock.NewManager(taskLockRepo, cfg.Timings.SubtaskLockTimeout)

	// Task usecases
	publisher := taskuc.NewPublisher(gameRepo, mediaStorage, bot, cfg, log)
	requestAnswerer := taskuc.NewRequestAnswerer(taskResponseRepo, playerStateRepo, bot, &cfg.Messages, &cfg.Timings, log)
	answerer := taskuc.NewAnswerer(taskResponseRepo, playerStateRepo, bot, &cfg.Messages, &cfg.Timings, log)
	skipper := taskuc.NewSkipper(taskResponseRepo, playerRepo, bot, &cfg.Messages, &cfg.Timings, log)

	// Subtask usecases
	votingCollageHandler := subtask.NewVotingCollageHandler(
		lockManager,
		subtaskProgressRepo,
		taskResponseRepo,
		playerStateRepo,
		mediaStorage,
		bot,
		&cfg.Messages,
		&cfg.Timings,
		log,
	)
	whoIsWhoHandler := subtask.NewWhoIsWhoHandler(
		lockManager,
		subtaskProgressRepo,
		taskResponseRepo,
		playerStateRepo,
		playerRepo,
		bot,
		&cfg.Messages,
		&cfg.Timings,
		log,
	)
	memeVoiceoverHandler := subtask.NewMemeVoiceoverHandler(
		lockManager,
		subtaskProgressRepo,
		taskResponseRepo,
		playerStateRepo,
		mediaStorage,
		bot,
		&cfg.Messages,
		&cfg.Timings,
		log,
	)
	pollHandler := subtask.NewPollHandler(gameRepo, taskResultRepo, bot, cfg, log)
	adminOnlyHandler := subtask.NewAdminOnlyHandler(
		subtaskProgressRepo,
		taskResponseRepo,
		taskResultRepo,
		playerStateRepo,
		openaiClient,
		bot,
		&cfg.Messages,
		&cfg.Timings,
		log,
	)

	// Game usecases
	creator := game.NewCreator(gameRepo, playerRepo, playerStateRepo, log)
	joiner := game.NewJoiner(gameRepo, playerRepo, playerStateRepo, bot, &cfg.Messages, &cfg.Timings, log)
	leaver := game.NewLeaver(playerRepo, bot, &cfg.Messages, &cfg.Timings, log)
	starter := game.NewStarter(gameRepo, mediaStorage, bot, publisher, cfg, log)

	// Handlers
	chatMemberHandler := handler.NewChatMemberHandler(creator, bot, cfg, log)
	callbackHandler := handler.NewCallbackHandler(joiner, leaver, starter, requestAnswerer, skipper, votingCollageHandler, whoIsWhoHandler, memeVoiceoverHandler, adminOnlyHandler, cfg, log)
	messageHandler := handler.NewMessageHandler(gameRepo, playerRepo, playerStateRepo, answerer, memeVoiceoverHandler, adminOnlyHandler, cfg, log)
	pollAnswerHandler := handler.NewPollAnswerHandler(pollHandler, log)

	// Middleware
	pc := botmw.PlayerCheck(gameRepo, playerRepo, bot, &cfg.Messages, &cfg.Timings, log)

	// Global middleware
	bot.Use(botmw.Recover(log))

	// Routes — game management
	bot.Handle(tele.OnMyChatMember, chatMemberHandler.OnMyChatMember)
	bot.Handle("\fgame:join", callbackHandler.OnJoin)
	bot.Handle("\fgame:leave", callbackHandler.OnLeave, pc)
	bot.Handle("\fgame:leave_confirm", callbackHandler.OnLeaveConfirm, pc)
	bot.Handle("\fgame:leave_cancel", callbackHandler.OnLeaveCancel, pc)
	bot.Handle("\fgame:start", callbackHandler.OnStart, pc)

	// Routes — task interactions
	bot.Handle("\ftask:request", callbackHandler.OnTaskRequestAnswer, pc)
	bot.Handle("\ftask:skip", callbackHandler.OnTaskSkip, pc)
	bot.Handle("\ftask02:choice", callbackHandler.OnTask02Choice, pc)
	bot.Handle("\ftask04:player", callbackHandler.OnTask04PlayerChoice, pc)
	bot.Handle("\ftask10:meme_request", callbackHandler.OnTask10MemeRequest, pc)
	bot.Handle("\ftask12:question", callbackHandler.OnTask12Question, pc)
	bot.Handle(tele.OnPoll, pollAnswerHandler.OnPoll)
	bot.Handle(tele.OnText, messageHandler.OnMessage)
	bot.Handle(tele.OnPhoto, messageHandler.OnMessage)
	bot.Handle(tele.OnVideo, messageHandler.OnMessage)
	bot.Handle(tele.OnAudio, messageHandler.OnMessage)
	bot.Handle(tele.OnVoice, messageHandler.OnMessage)
	bot.Handle(tele.OnVideoNote, messageHandler.OnMessage)
	bot.Handle(tele.OnDocument, messageHandler.OnMessage)

	log.Info().Msg("bot started")
	bot.Start()
}
