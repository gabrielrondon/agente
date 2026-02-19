package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	sqlite "modernc.org/sqlite"
	"google.golang.org/protobuf/proto"
)

func init() {
	// whatsmeow's sqlstore calls sql.Open("sqlite3", ...) internally.
	// Register modernc.org/sqlite (no CGO) under that name if not yet registered.
	for _, d := range sql.Drivers() {
		if d == "sqlite3" {
			return
		}
	}
	sql.Register("sqlite3", &sqlite.Driver{})
}

// RealSender implements MessageSender using whatsmeow (real WhatsApp Web).
type RealSender struct {
	client   *whatsmeow.Client
	handlers []func(from, msg string)
}

// NewRealSender connects to WhatsApp.
// On first run it shows a QR code; subsequent runs reuse the saved session.
// dbPath is the SQLite file for session persistence (e.g. "data/whatsapp.db").
func NewRealSender(ctx context.Context, dbPath string) (*RealSender, error) {
	container, err := sqlstore.New(ctx, "sqlite3", "file:"+dbPath+"?_foreign_keys=on", waLog.Noop)
	if err != nil {
		return nil, fmt.Errorf("whatsapp store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}

	r := &RealSender{}
	client := whatsmeow.NewClient(deviceStore, waLog.Noop)
	r.client = client

	// Register handler for incoming messages BEFORE connecting
	client.AddEventHandler(r.handleEvent)

	if client.Store.ID == nil {
		// First run: pair via QR code
		qrChan, err := client.GetQRChannel(ctx)
		if err != nil {
			return nil, fmt.Errorf("get qr channel: %w", err)
		}
		if err := client.Connect(); err != nil {
			return nil, fmt.Errorf("connect: %w", err)
		}

		fmt.Println("\n=== WhatsApp — Primeira Conexão ===")
		fmt.Println("Abra o WhatsApp > Dispositivos conectados > Conectar dispositivo")
		fmt.Println("Escaneie o QR code abaixo:\n")

		for item := range qrChan {
			switch item.Event {
			case "code":
				qrterminal.GenerateHalfBlock(item.Code, qrterminal.L, os.Stdout)
				fmt.Printf("(expira em %.0fs)\n", item.Timeout.Seconds())
			default:
				if item == whatsmeow.QRChannelSuccess {
					fmt.Println("\nConectado com sucesso!")
				} else if item == whatsmeow.QRChannelTimeout {
					return nil, fmt.Errorf("timeout aguardando QR scan")
				}
			}
		}
	} else {
		// Session already exists — reconnect
		if err := client.Connect(); err != nil {
			return nil, fmt.Errorf("reconectar: %w", err)
		}
		fmt.Printf("WhatsApp conectado: %s\n", client.Store.ID.User)
	}

	return r, nil
}

// Send sends a WhatsApp text message to the given phone number.
// phone format: international digits only, e.g. "5567999990000"
func (r *RealSender) Send(phone, message string) error {
	phone = normalizePhone(phone)
	jid := types.NewJID(phone, types.DefaultUserServer)

	_, err := r.client.SendMessage(context.Background(), jid, &waE2E.Message{
		Conversation: proto.String(message),
	})
	if err != nil {
		return fmt.Errorf("send to %s: %w", phone, err)
	}
	fmt.Printf("[WhatsApp enviado → %s]\n", phone)
	return nil
}

// Listen registers a handler called for every incoming private text message.
func (r *RealSender) Listen(handler func(from, msg string)) error {
	r.handlers = append(r.handlers, handler)
	return nil
}

// Close disconnects from WhatsApp.
func (r *RealSender) Close() error {
	r.client.Disconnect()
	return nil
}

func (r *RealSender) handleEvent(evt any) {
	msg, ok := evt.(*events.Message)
	if !ok {
		return
	}
	if msg.Info.IsFromMe || msg.Info.IsGroup {
		return
	}

	// Extract text — handles plain and extended text messages
	text := msg.Message.GetConversation()
	if text == "" && msg.Message.GetExtendedTextMessage() != nil {
		text = msg.Message.GetExtendedTextMessage().GetText()
	}
	if text == "" {
		return
	}

	from := msg.Info.Sender.User // phone number without @s.whatsapp.net
	for _, h := range r.handlers {
		h(from, text)
	}
}

// normalizePhone strips non-digit characters and removes leading "+".
func normalizePhone(phone string) string {
	phone = strings.TrimPrefix(phone, "+")
	var out strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			out.WriteRune(r)
		}
	}
	return out.String()
}
