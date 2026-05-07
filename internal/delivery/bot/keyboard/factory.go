package keyboard

import (
	"strconv"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	tele "gopkg.in/telebot.v3"
)

// JoinKeyboard returns the inline keyboard shown when the bot joins a chat.
func JoinKeyboard(supportURL string) *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	join := kbd.Data("Приєднатися до гри 🎮", "game:join")
	support := kbd.URL("Підтримка 💬", supportURL)
	kbd.Inline(kbd.Row(join), kbd.Row(support))
	return kbd
}

// StartKeyboard returns the inline keyboard with the "Start game" button.
func StartKeyboard() *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	start := kbd.Data("Розпочати гру 🚀", "game:start")
	kbd.Inline(kbd.Row(start))
	return kbd
}

// LeaveConfirmKeyboard returns the inline keyboard for leave confirmation.
func LeaveConfirmKeyboard() *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	yes := kbd.Data("Так, виходжу 👋", "game:leave_confirm")
	no := kbd.Data("Ні, залишаюсь 💕", "game:leave_cancel")
	kbd.Inline(kbd.Row(yes, no))
	return kbd
}

// TaskKeyboard returns the inline keyboard attached to a published task.
// taskID is passed as the callback payload; handlers receive it via c.Data().
func TaskKeyboard(taskID string) *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	answer := kbd.Data("Хочу відповісти ✍️", "task:request", taskID)
	skip := kbd.Data("Пропустити ⏭️", "task:skip", taskID)
	kbd.Inline(kbd.Row(answer, skip))
	return kbd
}

// PlayerSelectionKeyboard returns the inline keyboard for player selection in task_04.
// Each button carries callback data "questionID:telegramUserID" routed via "\ftask04:player".
func PlayerSelectionKeyboard(players []*entity.Player, questionID string) *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	buttons := make([]tele.Row, 0, len(players))
	for _, p := range players {
		label := p.FirstName
		if p.Username != "" {
			label = "@" + p.Username
		}
		payload := questionID + ":" + strconv.FormatInt(p.TelegramUserID, 10)
		btn := kbd.Data(label, "task04:player", payload)
		buttons = append(buttons, kbd.Row(btn))
	}
	kbd.Inline(buttons...)
	return kbd
}

// CategoryKeyboard returns the inline keyboard for a voting category in task_02.
// Each button carries callback data "categoryID:optionID" routed via "\ftask02:choice".
func CategoryKeyboard(task *config.Task, catIdx int) *tele.ReplyMarkup {
	cat := task.Subtask.Categories[catIdx]
	kbd := &tele.ReplyMarkup{}
	buttons := make([]tele.Row, 0, len(cat.Options))
	for _, opt := range cat.Options {
		payload := cat.ID + ":" + opt.ID
		btn := kbd.Data(opt.Label, "task02:choice", payload)
		buttons = append(buttons, kbd.Row(btn))
	}
	kbd.Inline(buttons...)
	return kbd
}
