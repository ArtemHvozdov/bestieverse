package finalize_test

import (
	"fmt"

	tele "gopkg.in/telebot.v3"
)

type mockSender struct {
	sent    []interface{}
	deleted int
}

func (m *mockSender) Send(_ tele.Recipient, what interface{}, _ ...interface{}) (*tele.Message, error) {
	m.sent = append(m.sent, what)
	return &tele.Message{ID: len(m.sent)}, nil
}

func (m *mockSender) Delete(_ tele.Editable) error {
	m.deleted++
	return nil
}

// noopMedia satisfies media.Storage and always returns an error so callers fall back to text.
type noopMedia struct{}

func (noopMedia) GetFile(_ string) (*tele.Document, error)   { return nil, fmt.Errorf("noop") }
func (noopMedia) GetPhoto(_ string) (*tele.Photo, error)     { return nil, fmt.Errorf("noop") }
func (noopMedia) GetAnimation(_ string) (*tele.Animation, error) { return nil, fmt.Errorf("noop") }
