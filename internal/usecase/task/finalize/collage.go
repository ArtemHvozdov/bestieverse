package finalize

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/jpeg"
	_ "image/png"
	"math"
	"os"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
	xdraw "golang.org/x/image/draw"
)

const (
	collageGridSize   = 3
	collageCellSize   = 720
	collageCanvasSize = collageCellSize * collageGridSize // 2160
)

// CollageFinalizer handles summary.type == "collage" (task_02).
// It tallies votes, assembles a 3×3 collage from winning images,
// and sends it to the chat.
type CollageFinalizer struct {
	taskResultRepo repository.TaskResultRepository
	media          media.Storage
	sender         Sender
	log            zerolog.Logger
}

func NewCollageFinalizer(
	taskResultRepo repository.TaskResultRepository,
	mediaStorage media.Storage,
	sender Sender,
	log zerolog.Logger,
) *CollageFinalizer {
	return &CollageFinalizer{
		taskResultRepo: taskResultRepo,
		media:          mediaStorage,
		sender:         sender,
		log:            log,
	}
}

func (f *CollageFinalizer) SupportedSummaryType() string { return SummaryTypeCollage }

func (f *CollageFinalizer) Finalize(
	ctx context.Context,
	game *entity.Game,
	task *config.Task,
	responses []*entity.TaskResponse,
) error {
	if task.Subtask == nil || len(task.Subtask.Categories) == 0 {
		return fmt.Errorf("collage.Finalize: task %s has no subtask categories", task.ID)
	}

	winners, err := f.determineWinners(task, responses)
	if err != nil {
		return fmt.Errorf("collage.Finalize: determine winners: %w", err)
	}

	resultData, _ := json.Marshal(winners)
	if err := f.taskResultRepo.Create(ctx, &entity.TaskResult{
		GameID:      game.ID,
		TaskID:      task.ID,
		ResultData:  resultData,
		FinalizedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("collage.Finalize: save result: %w", err)
	}

	chat := &tele.Chat{ID: game.ChatID}
	f.sender.Send(chat, task.Summary.PendingText, tele.ModeHTML) //nolint:errcheck

	collagePath, err := f.buildCollage(task, winners)
	if err != nil {
		f.log.Error().Err(err).Str("task", task.ID).Msg("collage: failed to build image")
		return fmt.Errorf("collage.Finalize: build collage: %w", err)
	}

	photoFile := tele.FromDisk(collagePath)
	photo := &tele.Photo{
		File:    photoFile,
		Caption: task.Summary.ReadyText,
	}
	f.sender.Send(chat, photo, tele.ModeHTML) //nolint:errcheck

	doc := &tele.Document{
		File:     tele.FromDisk(collagePath),
		MIME:     "image/jpeg",
		FileName: "collage_2160x2160.jpg",
		Caption:  task.Summary.HqText,
	}
	f.sender.Send(chat, doc, tele.ModeHTML) //nolint:errcheck

	go func() {
		time.Sleep(5 * time.Second)
		if err := os.Remove(collagePath); err != nil {
			f.log.Warn().Err(err).Str("path", collagePath).Msg("collage: temp file cleanup failed")
		}
	}()

	f.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Uint64("game", game.ID).
		Str("task", task.ID).
		Msg("collage finalized")

	return nil
}

// determineWinners counts votes per category and picks the winner.
// Tie-breaking: first option by YAML order wins.
func (f *CollageFinalizer) determineWinners(task *config.Task, responses []*entity.TaskResponse) (map[string]string, error) {
	// votes[categoryID][optionID] = count
	votes := make(map[string]map[string]int)
	for _, cat := range task.Subtask.Categories {
		votes[cat.ID] = make(map[string]int)
	}

	for _, resp := range responses {
		if resp.ResponseData == nil {
			continue
		}
		var choices map[string]string
		if err := json.Unmarshal(resp.ResponseData, &choices); err != nil {
			continue
		}
		for catID, optID := range choices {
			if _, ok := votes[catID]; ok {
				votes[catID][optID]++
			}
		}
	}

	winners := make(map[string]string, len(task.Subtask.Categories))
	for _, cat := range task.Subtask.Categories {
		catVotes := votes[cat.ID]
		winner := ""
		maxVotes := -1
		// Iterate in YAML order for deterministic tie-breaking.
		for _, opt := range cat.Options {
			count := catVotes[opt.ID]
			if count > maxVotes {
				maxVotes = count
				winner = opt.ID
			}
		}
		if winner == "" && len(cat.Options) > 0 {
			winner = cat.Options[0].ID
		}
		winners[cat.ID] = winner
	}

	return winners, nil
}

// buildCollage assembles a 3×3 JPEG collage from the winning option images.
// Returns the path to the temporary file.
func (f *CollageFinalizer) buildCollage(task *config.Task, winners map[string]string) (string, error) {
	// Collect winning image paths in category order (max 9 for 3×3).
	type imageEntry struct {
		mediaFile string
	}
	var entries []imageEntry
	for _, cat := range task.Subtask.Categories {
		optID := winners[cat.ID]
		for _, opt := range cat.Options {
			if opt.ID == optID {
				entries = append(entries, imageEntry{opt.MediaFile})
				break
			}
		}
		if len(entries) >= collageGridSize*collageGridSize {
			break
		}
	}

	// Build canvas with dark background.
	canvas := image.NewRGBA(image.Rect(0, 0, collageCanvasSize, collageCanvasSize))
	darkBG := image.NewUniform(color.RGBA{R: 13, G: 13, B: 13, A: 255})
	xdraw.Draw(canvas, canvas.Bounds(), darkBG, image.Point{}, xdraw.Src)

	for i, entry := range entries {
		img, err := f.loadImage(entry.mediaFile)
		if err != nil {
			f.log.Warn().Err(err).Str("file", entry.mediaFile).Msg("collage: skipping image")
			continue
		}

		tile := fitImageToTile(img, collageCellSize, collageCellSize)
		row := i / collageGridSize
		col := i % collageGridSize
		offset := image.Pt(col*collageCellSize, row*collageCellSize)
		xdraw.Draw(canvas, tile.Bounds().Add(offset), tile, image.Point{}, xdraw.Over)
	}

	tmp, err := os.CreateTemp(os.TempDir(), "collage_*.jpg")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmp.Close()

	if err := jpeg.Encode(tmp, canvas, &jpeg.Options{Quality: 95}); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("encode jpeg: %w", err)
	}

	return tmp.Name(), nil
}

// loadImage loads an image from media storage for collage assembly.
func (f *CollageFinalizer) loadImage(mediaFile string) (image.Image, error) {
	photo, err := f.media.GetPhoto(mediaFile)
	if err != nil {
		return nil, fmt.Errorf("get photo %s: %w", mediaFile, err)
	}
	file, err := os.Open(photo.FileLocal)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", photo.FileLocal, err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", photo.FileLocal, err)
	}
	return img, nil
}

// fitImageToTile scales img to fit within targetW×targetH while preserving aspect ratio,
// centered on a white background.
func fitImageToTile(img image.Image, targetW, targetH int) *image.RGBA {
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()

	scale := math.Min(float64(targetW)/float64(iw), float64(targetH)/float64(ih)) * 0.98
	nw := int(float64(iw) * scale)
	nh := int(float64(ih) * scale)

	resized := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), img, b, xdraw.Over, nil)

	tile := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	whiteBG := image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255})
	xdraw.Draw(tile, tile.Bounds(), whiteBG, image.Point{}, xdraw.Src)

	off := image.Pt((targetW-nw)/2, (targetH-nh)/2)
	xdraw.Draw(tile, resized.Bounds().Add(off), resized, image.Point{}, xdraw.Over)

	return tile
}
