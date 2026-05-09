package formatter

import (
	"bytes"
	"fmt"
	"text/template"

	tele "gopkg.in/telebot.v3"
)

const ParseMode = tele.ModeHTML

// Mention returns a Telegram mention string.
// If username is set, returns plain "@username" — Telegram auto-links it without underline styling.
// If username is empty, returns an HTML anchor with firstName so the user is still clickable.
func Mention(userID int64, username, firstName string) string {
	if username != "" {
		return "@" + username
	}
	return fmt.Sprintf(`<a href="tg://user?id=%d">%s</a>`, userID, firstName)
}

// RenderTemplate executes a Go text/template with the provided data.
func RenderTemplate(tmpl string, data any) (string, error) {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("formatter.RenderTemplate: parse: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("formatter.RenderTemplate: execute: %w", err)
	}
	return buf.String(), nil
}
