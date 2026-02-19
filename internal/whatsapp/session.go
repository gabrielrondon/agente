package whatsapp

// session.go is a placeholder for the real whatsmeow session persistence.
// When plugging in real WhatsApp, this file will handle:
//   - QR code scanning on first run
//   - Session storage in SQLite (whatsmeow's built-in store)
//   - Reconnection logic

// RealSender will implement MessageSender using whatsmeow.
// Uncomment and complete when ready to connect to real WhatsApp:
//
// import (
//   "go.mau.fi/whatsmeow"
//   "go.mau.fi/whatsmeow/store/sqlstore"
//   waLog "go.mau.fi/whatsmeow/util/log"
// )
//
// type RealSender struct {
//   client *whatsmeow.Client
// }
//
// func NewRealSender(dbPath string) (*RealSender, error) {
//   container, err := sqlstore.New("sqlite", "file:"+dbPath+"?_foreign_keys=on", waLog.Noop)
//   if err != nil { return nil, err }
//   deviceStore, err := container.GetFirstDevice()
//   if err != nil { return nil, err }
//   client := whatsmeow.NewClient(deviceStore, waLog.Noop)
//   if client.Store.ID == nil {
//     // First run: show QR code
//     qrChan, _ := client.GetQRChannel(context.Background())
//     go func() {
//       for evt := range qrChan {
//         if evt.Event == "code" { fmt.Println("QR:", evt.Code) }
//       }
//     }()
//   }
//   return &RealSender{client: client}, client.Connect()
// }
