package config

import (
	"fmt"
	"os"
	"time"
)

// Timings holds all configurable time intervals.
// Source depends on TEST_MODE: prod vars or TEST_* vars.
type Timings struct {
	// Messaging
	DeleteMessageDelay time.Duration // 10s — auto-delete temporary messages
	JoinMessageDelay   time.Duration // 1s  — delay before showing "Start game" button
	TaskInfoInterval   time.Duration // 1s  — interval between info messages

	// Subtask exclusive lock
	SubtaskLockTimeout time.Duration // 15m — lock timeout for exclusive subtasks

	// Scheduler
	TaskPublishInterval time.Duration // 24h (prod) — interval between task publications
	TaskFinalizeOffset  time.Duration // 23h (prod) — how long after publish before finalize
	PollDuration        time.Duration // duration of Telegram poll (task 10)

	// Notifier
	ReminderDelay time.Duration // 12h (prod) — delay before sending reminders
}

func loadTimings(testMode bool) (Timings, error) {
	if testMode {
		return loadTestTimings()
	}
	return loadProdTimings()
}

func loadProdTimings() (Timings, error) {
	deleteDelay, err := parseDurationEnv("DELETE_MESSAGE_DELAY", "10s")
	if err != nil {
		return Timings{}, fmt.Errorf("DELETE_MESSAGE_DELAY: %w", err)
	}
	joinDelay, err := parseDurationEnv("JOIN_MESSAGE_DELAY", "1s")
	if err != nil {
		return Timings{}, fmt.Errorf("JOIN_MESSAGE_DELAY: %w", err)
	}
	taskInfo, err := parseDurationEnv("TASK_INFO_INTERVAL", "1s")
	if err != nil {
		return Timings{}, fmt.Errorf("TASK_INFO_INTERVAL: %w", err)
	}
	lockTimeout, err := parseDurationEnv("SUBTASK_LOCK_TIMEOUT", "15m")
	if err != nil {
		return Timings{}, fmt.Errorf("SUBTASK_LOCK_TIMEOUT: %w", err)
	}
	publishInterval, err := parseDurationEnv("TASK_PUBLISH_INTERVAL", "24h")
	if err != nil {
		return Timings{}, fmt.Errorf("TASK_PUBLISH_INTERVAL: %w", err)
	}
	finalizeOffset, err := parseDurationEnv("TASK_FINALIZE_OFFSET", "23h")
	if err != nil {
		return Timings{}, fmt.Errorf("TASK_FINALIZE_OFFSET: %w", err)
	}
	pollDuration, err := parseDurationEnv("POLL_DURATION", "24h")
	if err != nil {
		return Timings{}, fmt.Errorf("POLL_DURATION: %w", err)
	}
	reminderDelay, err := parseDurationEnv("REMINDER_DELAY", "12h")
	if err != nil {
		return Timings{}, fmt.Errorf("REMINDER_DELAY: %w", err)
	}

	return Timings{
		DeleteMessageDelay:  deleteDelay,
		JoinMessageDelay:    joinDelay,
		TaskInfoInterval:    taskInfo,
		SubtaskLockTimeout:  lockTimeout,
		TaskPublishInterval: publishInterval,
		TaskFinalizeOffset:  finalizeOffset,
		PollDuration:        pollDuration,
		ReminderDelay:       reminderDelay,
	}, nil
}

func loadTestTimings() (Timings, error) {
	deleteDelay, err := parseDurationEnv("DELETE_MESSAGE_DELAY", "1s")
	if err != nil {
		return Timings{}, fmt.Errorf("DELETE_MESSAGE_DELAY: %w", err)
	}
	joinDelay, err := parseDurationEnv("JOIN_MESSAGE_DELAY", "500ms")
	if err != nil {
		return Timings{}, fmt.Errorf("JOIN_MESSAGE_DELAY: %w", err)
	}
	taskInfo, err := parseDurationEnv("TASK_INFO_INTERVAL", "500ms")
	if err != nil {
		return Timings{}, fmt.Errorf("TASK_INFO_INTERVAL: %w", err)
	}
	lockTimeout, err := parseDurationEnv("SUBTASK_LOCK_TIMEOUT", "1m")
	if err != nil {
		return Timings{}, fmt.Errorf("SUBTASK_LOCK_TIMEOUT: %w", err)
	}
	publishInterval, err := parseDurationEnv("TEST_TASK_PUBLISH_INTERVAL", "2m")
	if err != nil {
		return Timings{}, fmt.Errorf("TEST_TASK_PUBLISH_INTERVAL: %w", err)
	}
	finalizeOffset, err := parseDurationEnv("TEST_TASK_FINALIZE_OFFSET", "1m50s")
	if err != nil {
		return Timings{}, fmt.Errorf("TEST_TASK_FINALIZE_OFFSET: %w", err)
	}
	pollDuration, err := parseDurationEnv("TEST_POLL_DURATION", "1m")
	if err != nil {
		return Timings{}, fmt.Errorf("TEST_POLL_DURATION: %w", err)
	}
	reminderDelay, err := parseDurationEnv("TEST_REMINDER_DELAY", "30s")
	if err != nil {
		return Timings{}, fmt.Errorf("TEST_REMINDER_DELAY: %w", err)
	}

	return Timings{
		DeleteMessageDelay:  deleteDelay,
		JoinMessageDelay:    joinDelay,
		TaskInfoInterval:    taskInfo,
		SubtaskLockTimeout:  lockTimeout,
		TaskPublishInterval: publishInterval,
		TaskFinalizeOffset:  finalizeOffset,
		PollDuration:        pollDuration,
		ReminderDelay:       reminderDelay,
	}, nil
}

// parseDurationEnv reads the env var and parses it as a duration.
// Falls back to defaultVal if the var is not set.
func parseDurationEnv(key, defaultVal string) (time.Duration, error) {
	val := os.Getenv(key)
	if val == "" {
		val = defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", val, err)
	}
	return d, nil
}
