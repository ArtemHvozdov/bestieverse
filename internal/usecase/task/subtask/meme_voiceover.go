package subtask

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/lock"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

const defaultMemesPerPlayer = 5

// memeFloodRetrySlack is added on top of Telegram's RetryAfter to give the
// flood window a moment to actually drain before we retry once.
const memeFloodRetrySlack = 1 * time.Second

// MemeVoiceoverHandler handles the sequential meme voiceover subtask (task_10b).
// Only one player can voice memes at a time — enforced by the exclusive lock.
// Player's voiceover answers are NOT deleted — they stay in chat as public content.
// Meme files are divided into fixed-size slots (memes_per_player from YAML, default 5):
// player 1 gets memes 1-5, player 2 gets memes 6-10, etc., in order of lock acquisition.
type MemeVoiceoverHandler struct {
	lockManager         *lock.Manager
	subtaskProgressRepo repository.SubtaskProgressRepository
	taskResponseRepo    repository.TaskResponseRepository
	playerStateRepo     repository.PlayerStateRepository
	media               media.Storage
	sender              Sender
	msgs                *config.Messages
	timings             *config.Timings
	log                 zerolog.Logger

	// coldUploadMu guards lastColdUpload so that the minimum spacing between
	// cold-cache GIF uploads is enforced globally across all concurrent players
	// and games — not just within a single player's session.
	coldUploadMu   sync.Mutex
	lastColdUpload time.Time
}

func NewMemeVoiceoverHandler(
	lockManager *lock.Manager,
	subtaskProgressRepo repository.SubtaskProgressRepository,
	taskResponseRepo repository.TaskResponseRepository,
	playerStateRepo repository.PlayerStateRepository,
	mediaStorage media.Storage,
	sender Sender,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) *MemeVoiceoverHandler {
	return &MemeVoiceoverHandler{
		lockManager:         lockManager,
		subtaskProgressRepo: subtaskProgressRepo,
		taskResponseRepo:    taskResponseRepo,
		playerStateRepo:     playerStateRepo,
		media:               mediaStorage,
		sender:              sender,
		msgs:                msgs,
		timings:             timings,
		log:                 log,
	}
}

// HandleRequestAnswer is called when a player presses the meme voiceover start button.
// Acquires the exclusive lock and sends the first meme GIF for this player's slot.
func (h *MemeVoiceoverHandler) HandleRequestAnswer(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	existing, err := h.taskResponseRepo.GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: get response: %w", err)
	}
	if existing != nil {
		text, _ := formatter.RenderTemplate(config.Random(h.msgs.AlreadyAnswered), struct{ Mention string }{mention})
		msg, _ := h.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(h.sender, msg, h.timings.DeleteMessageDelay)
		}
		return nil
	}

	acquired, err := h.lockManager.TryAcquire(ctx, game.ID, task.ID, player.ID)
	if err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: acquire lock: %w", err)
	}
	if !acquired {
		text, _ := formatter.RenderTemplate(h.msgs.SubtaskLocked, struct{ Mention string }{mention})
		msg, _ := h.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(h.sender, msg, h.timings.DeleteMessageDelay)
		}
		return nil
	}

	memeOpt := memeOptionFromTask(task)
	if memeOpt == nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: no meme_voiceover option in task %s", task.ID)
	}
	memeFiles := memeOpt.MemeFiles
	memesPerPlayer := memeOpt.MemesPerPlayer
	if memesPerPlayer == 0 {
		memesPerPlayer = defaultMemesPerPlayer
	}

	progress, err := h.subtaskProgressRepo.Get(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: get progress: %w", err)
	}
	if progress == nil {
		// Determine slot: count players who already completed their voiceover.
		answeredCount, err := h.taskResponseRepo.CountAnsweredByTask(ctx, game.ID, task.ID)
		if err != nil {
			return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: count answered: %w", err)
		}
		startIndex := answeredCount * memesPerPlayer

		initialData := map[string]string{
			"_start": strconv.Itoa(startIndex),
			"_per":   strconv.Itoa(memesPerPlayer),
		}
		initialAnswers, _ := json.Marshal(initialData)
		progress = &entity.SubtaskProgress{
			GameID:        game.ID,
			PlayerID:      player.ID,
			TaskID:        task.ID,
			QuestionIndex: 0,
			AnswersData:   initialAnswers,
		}
		if err := h.subtaskProgressRepo.Upsert(ctx, progress); err != nil {
			return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: create progress: %w", err)
		}
	}

	// Parse slot assignment from progress metadata.
	var meta map[string]string
	if err := json.Unmarshal(progress.AnswersData, &meta); err != nil {
		meta = make(map[string]string)
	}
	startIndex, _ := strconv.Atoi(meta["_start"])
	currentIdx := startIndex + progress.QuestionIndex

	// TaskID with ":meme" suffix routes message.go to this handler for all incoming messages.
	state := &entity.PlayerState{
		GameID:   game.ID,
		PlayerID: player.ID,
		State:    entity.PlayerStateAwaitingAnswer,
		TaskID:   task.ID + ":meme",
	}
	if err := h.playerStateRepo.Upsert(ctx, state); err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: upsert state: %w", err)
	}

	if currentIdx >= len(memeFiles) {
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: meme index %d out of range (total %d)", currentIdx, len(memeFiles))
	}
	if err := h.sendMeme(chat, memeFiles[currentIdx]); err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: send first meme: %w", err)
	}

	h.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
		Str("task", task.ID).
		Msg("meme_voiceover: lock acquired, first meme sent")

	return nil
}

// HandleAnswer processes the player's voiceover for the current meme.
// The voiceover message is NOT deleted — it stays in chat as public content.
// Any message type is accepted (text, voice, video, photo, etc.).
func (h *MemeVoiceoverHandler) HandleAnswer(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
	msg *tele.Message,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	h.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
		Str("task", task.ID).
		Msg("meme_voiceover.HandleAnswer: called")

	lockHolder, err := h.lockManager.TryAcquire(ctx, game.ID, task.ID, player.ID)
	if err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: check lock: %w", err)
	}
	if !lockHolder {
		h.log.Warn().
			Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
			Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
			Str("task", task.ID).
			Msg("meme_voiceover.HandleAnswer: lock not held by player, ignoring message")
		return nil
	}

	progress, err := h.subtaskProgressRepo.Get(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: get progress: %w", err)
	}
	if progress == nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: no progress for player %d", player.ID)
	}

	var answers map[string]string
	if err := json.Unmarshal(progress.AnswersData, &answers); err != nil {
		answers = make(map[string]string)
	}

	// Read slot metadata stored when the player started.
	memeOpt := memeOptionFromTask(task)
	memeFiles := memeOpt.MemeFiles
	startIndex, _ := strconv.Atoi(answers["_start"])
	memesPerPlayer, _ := strconv.Atoi(answers["_per"])
	if memesPerPlayer == 0 {
		memesPerPlayer = defaultMemesPerPlayer
	}

	memeKey := fmt.Sprintf("meme_%d", progress.QuestionIndex+1)
	answers[memeKey] = msg.Text

	nextQuestionIndex := progress.QuestionIndex + 1

	if nextQuestionIndex < memesPerPlayer {
		// Send next meme BEFORE saving progress so a failed send leaves progress
		// unchanged and the player can retry by sending any message again.
		nextIdx := startIndex + nextQuestionIndex
		if nextIdx >= len(memeFiles) {
			return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: meme index %d out of range (total %d)", nextIdx, len(memeFiles))
		}
		if err := h.sendMeme(chat, memeFiles[nextIdx]); err != nil {
			return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: send next meme: %w", err)
		}
		updated, _ := json.Marshal(answers)
		progress.AnswersData = updated
		progress.QuestionIndex = nextQuestionIndex
		if err := h.subtaskProgressRepo.Upsert(ctx, progress); err != nil {
			return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: upsert progress: %w", err)
		}
		h.log.Info().
			Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
			Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
			Str("task", task.ID).
			Int("meme_index", nextIdx).
			Int("question_index", nextQuestionIndex).
			Int("memes_per_player", memesPerPlayer).
			Msg("meme_voiceover: sent next meme")
		return nil
	}

	// All memes voiced — finalize.
	responseData, _ := json.Marshal(answers)
	resp := &entity.TaskResponse{
		GameID:       game.ID,
		PlayerID:     player.ID,
		TaskID:       task.ID,
		Status:       entity.ResponseAnswered,
		ResponseData: responseData,
	}
	if err := h.taskResponseRepo.Create(ctx, resp); err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: create response: %w", err)
	}

	if err := h.subtaskProgressRepo.Delete(ctx, game.ID, player.ID, task.ID); err != nil {
		h.log.Warn().Err(err).Msg("meme_voiceover: failed to delete progress")
	}

	if err := h.lockManager.Release(ctx, game.ID, task.ID); err != nil {
		h.log.Warn().Err(err).Msg("meme_voiceover: failed to release lock")
	}

	if err := h.playerStateRepo.SetIdle(ctx, game.ID, player.ID); err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: set idle: %w", err)
	}

	text, renderErr := formatter.RenderTemplate(config.Random(h.msgs.MemeVoiceoverDone), struct{ Mention string }{mention})
	if renderErr != nil {
		text = config.Random(h.msgs.MemeVoiceoverDone)
	}
	h.sender.Send(chat, text, formatter.ParseMode) //nolint:errcheck

	h.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
		Str("task", task.ID).
		Msg("meme_voiceover: all memes voiced")

	return nil
}

// waitColdGap enforces a global minimum spacing between cold-cache GIF uploads.
// Unlike a per-call Sleep it accounts for elapsed time since the previous upload,
// so a player who takes longer than the configured gap to write their voiceover
// experiences no extra delay. The very first cold upload per handler lifetime
// proceeds without any wait at all.
//
// The lock is held for the duration of any sleep to serialise concurrent callers
// — this is intentional: we want all cold uploads, across all concurrent players
// and games, to be spaced at least MemeColdCacheDelay apart.
func (h *MemeVoiceoverHandler) waitColdGap() {
	if h.timings.MemeColdCacheDelay <= 0 {
		return
	}
	h.coldUploadMu.Lock()
	defer h.coldUploadMu.Unlock()
	if !h.lastColdUpload.IsZero() {
		if remaining := h.timings.MemeColdCacheDelay - time.Since(h.lastColdUpload); remaining > 0 {
			time.Sleep(remaining)
		}
	}
	h.lastColdUpload = time.Now()
}

// sendMeme sends a single meme GIF to the chat without any text.
//
// Once Telegram returns the file_id after the first successful upload, it is
// cached in media.Storage so subsequent sends of the same meme skip multipart
// upload entirely — this is the main lever for avoiding 429 flood errors when
// many GIFs are sent to the same chat in a short window.
//
// On a 429 (telebot.FloodError) we honour Telegram's RetryAfter and retry once.
// This handles the first cold-cache run where some uploads can still trip the
// limit before file_ids have been cached.
func (h *MemeVoiceoverHandler) sendMeme(chat *tele.Chat, memeFile string) error {
	anim, err := h.media.GetAnimation(memeFile)
	if err != nil {
		return fmt.Errorf("get animation %s: %w", memeFile, err)
	}

	// Cold cache (file is about to be uploaded from disk): apply the global
	// rate limiter so that back-to-back cold uploads — including uploads across
	// the player-session boundary — stay under Telegram's per-chat media limit.
	// Cached sends (FileID already known) skip this entirely. anim may be nil
	// in tests where the media stub returns (nil, nil).
	if anim != nil && anim.File.FileID == "" {
		h.waitColdGap()
	}

	msg, err := h.sender.Send(chat, anim)
	if err != nil {
		var flood tele.FloodError
		if errors.As(err, &flood) {
			wait := time.Duration(flood.RetryAfter)*time.Second + memeFloodRetrySlack
			h.log.Warn().
				Int("retry_after", flood.RetryAfter).
				Str("meme", memeFile).
				Msg("meme_voiceover: flood limit hit, sleeping before retry")
			time.Sleep(wait)
			anim, err = h.media.GetAnimation(memeFile)
			if err != nil {
				return fmt.Errorf("get animation %s (retry): %w", memeFile, err)
			}
			msg, err = h.sender.Send(chat, anim)
		}
		if err != nil {
			return fmt.Errorf("send animation %s: %w", memeFile, err)
		}
	}

	if msg != nil && msg.Animation != nil && msg.Animation.FileID != "" {
		h.media.CacheFileID(memeFile, msg.Animation.FileID)
	}
	return nil
}

// memeOptionFromTask returns the poll option with result_type=meme_voiceover.
func memeOptionFromTask(task *config.Task) *config.PollOption {
	if task.Poll == nil {
		return nil
	}
	for i := range task.Poll.Options {
		if task.Poll.Options[i].ResultType == "meme_voiceover" {
			return &task.Poll.Options[i]
		}
	}
	return nil
}
