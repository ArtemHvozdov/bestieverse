package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRandom_SingleElement(t *testing.T) {
	result := Random([]string{"only"})
	assert.Equal(t, "only", result)
}

func TestRandom_EmptySlice(t *testing.T) {
	result := Random([]string{})
	assert.Equal(t, "", result)
}

func TestRandom_MultipleElements_ReturnsOneOf(t *testing.T) {
	variants := []string{"a", "b", "c"}
	result := Random(variants)
	assert.Contains(t, variants, result)
}

func TestRandom_MultipleElements_IsVariative(t *testing.T) {
	// Run enough times to confirm we don't always get the same element.
	variants := []string{"a", "b", "c", "d", "e"}
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		seen[Random(variants)] = true
	}
	// With 5 options and 100 tries the probability of hitting fewer than 2 unique values is negligible.
	assert.Greater(t, len(seen), 1)
}

func TestMessages_YAMLUnmarshal(t *testing.T) {
	raw := `
bot_message_start: "start message"
join_welcome:
  - "welcome 1"
  - "welcome 2"
join_already_member: "already member"
skip_with_remaining_2: "2 left"
skip_with_remaining_1: "1 left"
skip_last: "0 left"
skip_no_remaining: "no skips"
na_answers:
  - "no answers 1"
  - "no answers 2"
reminder:
  - "reminder 1"
`
	var msgs Messages
	require.NoError(t, yaml.Unmarshal([]byte(raw), &msgs))

	assert.Equal(t, "start message", msgs.BotMessageStart)
	assert.Equal(t, []string{"welcome 1", "welcome 2"}, msgs.JoinWelcome)
	assert.Equal(t, "already member", msgs.JoinAlreadyMember)
	assert.Equal(t, "2 left", msgs.SkipWithRemaining2)
	assert.Len(t, msgs.NaAnswers, 2)
	assert.Len(t, msgs.Reminder, 1)
}
