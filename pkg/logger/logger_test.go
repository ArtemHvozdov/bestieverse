package logger_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		log := logger.New("debug", "")
		log.Debug().Msg("test")
	})
}

func TestNew_InvalidLevelFallsBackToInfo(t *testing.T) {
	assert.NotPanics(t, func() {
		log := logger.New("invalid_level", "")
		log.Info().Msg("test")
	})
}

func TestWithChat_AddsFieldToOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.NewWithWriter(buf, "debug")

	chatLog := logger.WithChat(log, 123456789)
	chatLog.Info().Msg("chat test")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.EqualValues(t, 123456789, entry["chat"])
}

func TestWithUser_AddsFieldToOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.NewWithWriter(buf, "debug")

	userLog := logger.WithUser(log, 987654321, "@testuser")
	userLog.Info().Msg("user test")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "( 987654321 | @testuser)", entry["user"])
}

func TestWithUser_EmptyUsername(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.NewWithWriter(buf, "debug")

	userLog := logger.WithUser(log, 987654321, "")
	userLog.Info().Msg("user test")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "( 987654321 | )", entry["user"])
}

func TestWithChat_WithUser_Combined(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.NewWithWriter(buf, "debug")

	combined := logger.WithUser(logger.WithChat(log, 111), 222, "@foo")
	combined.Info().Msg("combined")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.EqualValues(t, 111, entry["chat"])
	assert.Equal(t, "( 222 | @foo)", entry["user"])
}
