package game_test

import tele "gopkg.in/telebot.v3"

type mockSender struct {
	messages []interface{}
	deleted  int
}

func (m *mockSender) Send(_ tele.Recipient, what interface{}, _ ...interface{}) (*tele.Message, error) {
	m.messages = append(m.messages, what)
	return &tele.Message{ID: len(m.messages)}, nil
}

func (m *mockSender) Delete(_ tele.Editable) error {
	m.deleted++
	return nil
}
