package formatter_test

import (
	"testing"

	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMention_WithUsername(t *testing.T) {
	result := formatter.Mention(123, "testuser", "Test")
	assert.Equal(t, "@testuser", result)
}

func TestMention_WithoutUsername(t *testing.T) {
	result := formatter.Mention(123, "", "FirstName")
	assert.Equal(t, `<a href="tg://user?id=123">FirstName</a>`, result)
}

func TestRenderTemplate_Success(t *testing.T) {
	type data struct{ Mention string }
	result, err := formatter.RenderTemplate("Привіт {{.Mention}}", data{Mention: "@user"})
	require.NoError(t, err)
	assert.Equal(t, "Привіт @user", result)
}

func TestRenderTemplate_SyntaxError(t *testing.T) {
	_, err := formatter.RenderTemplate("{{invalid", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}
