package formatter

import (
	"bytes"
	"fmt"
	"text/template"

	tele "gopkg.in/telebot.v3"
)

const ParseMode = tele.ModeHTML

// Mention returns an HTML anchor tag that mentions a Telegram user.
// If username is empty, firstName is used as the display name.
func Mention(userID int64, username, firstName string) string {
	name := firstName
	if username != "" {
		name = "@" + username
	}
	return fmt.Sprintf(`<a href="tg://user?id=%d">%s</a>`, userID, name)
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
