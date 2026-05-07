package config

import "math/rand"

// Messages holds all user-facing text templates loaded from content/messages.yaml.
// Fields with []string are variative — pick one with Random().
type Messages struct {
	BotMessageStart string `yaml:"bot_message_start"`

	JoinInvite        string   `yaml:"join_invite"`
	StartInvite       string   `yaml:"start_invite"`
	JoinWelcome       []string `yaml:"join_welcome"`
	JoinAlreadyMember string   `yaml:"join_already_member"`
	JoinAdminAlready  string   `yaml:"join_admin_already"`

	LeaveConfirm      string `yaml:"leave_confirm"`
	LeaveSuccess      string `yaml:"leave_success"`
	CancelLeave       string `yaml:"cancel_leave"`
	LeaveAdminBlocked string `yaml:"leave_admin_blocked"`

	StartOnlyAdmin string `yaml:"start_only_admin"`

	AwaitingAnswer []string `yaml:"awaiting_answer"`
	AnswerAccepted string   `yaml:"answer_accepted"`
	AlreadyAnswered []string `yaml:"already_answered"`

	SkipWithRemaining2 string `yaml:"skip_with_remaining_2"`
	SkipWithRemaining1 string `yaml:"skip_with_remaining_1"`
	SkipLast           string `yaml:"skip_last"`
	SkipNoRemaining    string `yaml:"skip_no_remaining"`
	AlreadySkipped     string `yaml:"already_skipped"`

	NotInGame     string `yaml:"not_in_game"`
	SubtaskLocked string `yaml:"subtask_locked"`

	NaAnswers []string `yaml:"na_answers"`
	Reminder  []string `yaml:"reminder"`

	Task12OnlyAdmin      string   `yaml:"task12_only_admin"`
	Task12AwaitingAnswer []string `yaml:"task12_awaiting_answer"`
	Task12Reply          []string `yaml:"task12_reply"`

	MemeVoiceoverAnnounce string   `yaml:"meme_voiceover_announce"`
	MemeVoiceoverDone     []string `yaml:"meme_voiceover_done"`
}

// Random returns a uniformly random element from variants.
// Returns empty string if variants is empty.
func Random(variants []string) string {
	if len(variants) == 0 {
		return ""
	}
	return variants[rand.Intn(len(variants))]
}
