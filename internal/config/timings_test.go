package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProdTimings_UsesEnvVars(t *testing.T) {
	t.Setenv("TASK_PUBLISH_INTERVAL", "24h")
	t.Setenv("TASK_FINALIZE_OFFSET", "23h")
	t.Setenv("REMINDER_DELAY", "12h")
	t.Setenv("SUBTASK_LOCK_TIMEOUT", "15m")
	t.Setenv("DELETE_MESSAGE_DELAY", "10s")

	timings, err := loadTimings(false)
	require.NoError(t, err)

	assert.Equal(t, 24*time.Hour, timings.TaskPublishInterval)
	assert.Equal(t, 23*time.Hour, timings.TaskFinalizeOffset)
	assert.Equal(t, 12*time.Hour, timings.ReminderDelay)
	assert.Equal(t, 15*time.Minute, timings.SubtaskLockTimeout)
	assert.Equal(t, 10*time.Second, timings.DeleteMessageDelay)
}

func TestLoadTestTimings_UsesTestEnvVars(t *testing.T) {
	t.Setenv("TEST_TASK_PUBLISH_INTERVAL", "2m")
	t.Setenv("TEST_TASK_FINALIZE_OFFSET", "1m50s")
	t.Setenv("TEST_REMINDER_DELAY", "30s")
	t.Setenv("TEST_POLL_DURATION", "1m")

	timings, err := loadTimings(true)
	require.NoError(t, err)

	assert.Equal(t, 2*time.Minute, timings.TaskPublishInterval)
	assert.Equal(t, 110*time.Second, timings.TaskFinalizeOffset)
	assert.Equal(t, 30*time.Second, timings.ReminderDelay)
	assert.Equal(t, time.Minute, timings.PollDuration)
}

func TestLoadProdTimings_UsesDefaultsWhenEnvNotSet(t *testing.T) {
	// Ensure vars are unset
	t.Setenv("TASK_PUBLISH_INTERVAL", "")
	t.Setenv("TASK_FINALIZE_OFFSET", "")
	t.Setenv("REMINDER_DELAY", "")

	timings, err := loadTimings(false)
	require.NoError(t, err)

	assert.Equal(t, 24*time.Hour, timings.TaskPublishInterval)
	assert.Equal(t, 23*time.Hour, timings.TaskFinalizeOffset)
	assert.Equal(t, 12*time.Hour, timings.ReminderDelay)
}

func TestParseDurationEnv_InvalidValue_ReturnsError(t *testing.T) {
	t.Setenv("DELETE_MESSAGE_DELAY", "not-a-duration")

	_, err := loadTimings(false)
	assert.Error(t, err)
}
