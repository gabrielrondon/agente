package whatsapp

import (
	"fmt"
	"time"
)

// MessageSender abstracts WhatsApp message delivery.
// MockSender is used in dev/dry-run; WhatsAppSender (whatsmeow) in production.
type MessageSender interface {
	Send(phone, message string) error
	Listen(handler func(from, msg string)) error
	Close() error
}

// IncomingMessage represents a received WhatsApp message.
type IncomingMessage struct {
	From      string
	Body      string
	Timestamp time.Time
}

// --- Mock implementation ---

// MockSender prints messages to stdout and records them in memory.
type MockSender struct {
	Sent     []SentMessage
	handlers []func(from, msg string)
}

// SentMessage records a message sent via MockSender.
type SentMessage struct {
	Phone   string
	Message string
	SentAt  time.Time
}

// NewMockSender creates a dry-run WhatsApp sender.
func NewMockSender() *MockSender {
	return &MockSender{}
}

func (m *MockSender) Send(phone, message string) error {
	msg := SentMessage{Phone: phone, Message: message, SentAt: time.Now()}
	m.Sent = append(m.Sent, msg)
	fmt.Printf("\n[DRY-RUN WhatsApp → %s]\n%s\n[/WhatsApp]\n\n", phone, message)
	return nil
}

func (m *MockSender) Listen(handler func(from, msg string)) error {
	m.handlers = append(m.handlers, handler)
	return nil
}

// SimulateReply injects a fake incoming message (used in tests/demos).
func (m *MockSender) SimulateReply(from, msg string) {
	for _, h := range m.handlers {
		h(from, msg)
	}
}

func (m *MockSender) Close() error { return nil }
