package config

// Task represents a single game task loaded from content/tasks/task_NN.yaml.
type Task struct {
	ID        string      `yaml:"id"`
	Order     int         `yaml:"order"`
	Type      string      `yaml:"type"`
	MediaFile string      `yaml:"media_file"`
	Text      string      `yaml:"text"`
	Summary   TaskSummary `yaml:"summary"`

	// Subtask configuration for voting_collage (task_02).
	Subtask *SubtaskVotingCollage `yaml:"subtask"`

	// Follow-up messages sent after each player completes the subtask (task_02).
	Followup []string `yaml:"followup"`

	// Questions for who_is_who (task_04) and admin_only (task_12).
	Questions []TaskQuestion `yaml:"questions"`

	// Poll configuration for poll_then_task (task_10).
	// Note: the YAML key is "pol" (original typo preserved for compatibility).
	Poll *SubtaskPoll `yaml:"pol"`

	// Multi-message publishing for admin_only (task_12).
	Messages []TaskMessage `yaml:"messages"`

	// OpenAI image generation config for task_12.
	OpenAI *TaskOpenAI `yaml:"openai"`
}

// TaskSummary holds the finalization config for a task.
type TaskSummary struct {
	Type        string   `yaml:"type"`
	Text        string   `yaml:"text"`
	HeaderText  string   `yaml:"header_text"`
	Predictions []string `yaml:"predictions"`

	// For voting_collage (type: collage)
	PendingText string `yaml:"pending_text"`
	ReadyText   string `yaml:"ready_text"`
	HqText      string `yaml:"hq_text"`

	// For admin_only (type: openai_collage)
	SendingText string `yaml:"sending_text"`
}

// SubtaskVotingCollage is the subtask config for task_02.
type SubtaskVotingCollage struct {
	Type          string           `yaml:"type"`
	ExclusiveLock bool             `yaml:"exclusive_lock"`
	Categories    []VotingCategory `yaml:"categories"`
}

// VotingCategory is one voting round within the voting_collage subtask.
type VotingCategory struct {
	ID         string          `yaml:"id"`
	HeaderText string          `yaml:"header_text"`
	MediaFile  string          `yaml:"media_file"`
	Options    []VotingOption  `yaml:"options"`
}

// VotingOption is a single selectable option within a VotingCategory.
type VotingOption struct {
	ID        string `yaml:"id"`
	Label     string `yaml:"label"`
	MediaFile string `yaml:"media_file"`
}

// TaskQuestion is a single question within who_is_who (task_04) or admin_only (task_12).
type TaskQuestion struct {
	ID          string `yaml:"id"`
	Text        string `yaml:"text"`
	ButtonLabel string `yaml:"button_label"`
}

// SubtaskPoll holds the Telegram poll config for task_10.
type SubtaskPoll struct {
	Title   string       `yaml:"title"`
	Options []PollOption `yaml:"options"`
}

// PollOption is one option in the Telegram poll.
// Depending on result_type it triggers a different follow-up subtask.
type PollOption struct {
	ID           string   `yaml:"id"`
	Label        string   `yaml:"label"`
	ResultType   string   `yaml:"result_type"` // question_answer | meme_voiceover
	PreparedText string   `yaml:"prepared_text"`
	MemeFiles    []string `yaml:"meme_files"`
}

// TaskMessage is one message to send when publishing task_12.
type TaskMessage struct {
	Type      string `yaml:"type"` // photo | text_with_buttons
	MediaFile string `yaml:"media_file"`
	Text      string `yaml:"text"`
}

// TaskOpenAI holds the prompt template for OpenAI image generation (task_12).
type TaskOpenAI struct {
	PromptTemplate string `yaml:"prompt_template"`
}

// GameMessages holds the intro and final messages loaded from content/game.yml.
type GameMessages struct {
	StartMessage1 string `yaml:"start_game_message_1"`
	StartMessage2 string `yaml:"start_game_message_2"`
	FinalMessage1 string `yaml:"final_message_1"`
	FinalMessage2 string `yaml:"final_message_2"`
}
