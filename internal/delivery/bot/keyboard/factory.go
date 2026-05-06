package keyboard

import tele "gopkg.in/telebot.v3"

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
func TaskKeyboard(taskID string) *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	answer := kbd.Data("Хочу відповісти ✍️", "task:request:"+taskID)
	skip := kbd.Data("Пропустити ⏭️", "task:skip:"+taskID)
	kbd.Inline(kbd.Row(answer, skip))
	return kbd
}
