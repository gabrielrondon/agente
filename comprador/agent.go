package comprador

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/user/agente/comprador/memory"
	"github.com/user/agente/comprador/suppliers"
	"github.com/user/agente/internal/claude"
	"github.com/user/agente/internal/whatsapp"
)

// Config holds agent configuration.
type Config struct {
	City         string
	QuoteTimeout time.Duration
	DryRun       bool
	WhatsAppDB   string // path for whatsmeow session DB
	OwnerPhone   string // if set, sends WhatsApp notification to owner when quotes are ready
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		City:         "local",
		QuoteTimeout: 30 * time.Minute,
		DryRun:       true,
		WhatsAppDB:   "data/whatsapp.db",
	}
}

// Agent is the main Comprador orchestrator.
type Agent struct {
	cfg      Config
	claude   *claude.Client
	sender   whatsapp.MessageSender
	supStore *suppliers.Store
	qStore   *suppliers.QuoteStore
	matcher  *suppliers.Matcher
	qManager *QuoteManager
	memStore *memory.Store
}

// New creates a new Comprador agent.
// If cfg.DryRun is false, pass a real whatsapp.MessageSender via WithSender or
// call NewWithWhatsApp to auto-connect via whatsmeow.
func New(db *sql.DB, cl *claude.Client, cfg Config) *Agent {
	sender := whatsapp.MessageSender(whatsapp.NewMockSender())

	supStore := suppliers.NewStore(db)
	qStore := suppliers.NewQuoteStore(db)
	matcher := suppliers.NewMatcher(cl, supStore)
	qManager := NewQuoteManager(cl, sender, supStore, qStore)
	memStore := memory.NewStore(db)

	return &Agent{
		cfg:      cfg,
		claude:   cl,
		sender:   sender,
		supStore: supStore,
		qStore:   qStore,
		matcher:  matcher,
		qManager: qManager,
		memStore: memStore,
	}
}

// SetSender swaps the WhatsApp sender (used to inject the real sender after QR login).
func (a *Agent) SetSender(s whatsapp.MessageSender) {
	a.sender = s
	a.qManager = NewQuoteManager(a.claude, s, a.supStore, a.qStore)

	// Register response handler: when a supplier replies, update the quote in the DB
	_ = s.Listen(func(from, msg string) {
		a.handleIncoming(from, msg)
	})
}

// handleIncoming matches an incoming WhatsApp message to a supplier quote and records it.
func (a *Agent) handleIncoming(from, msg string) {
	sup, err := a.supStore.ByPhone(from)
	if err != nil || sup == nil {
		return // unknown sender — ignore
	}
	fmt.Printf("\n[WhatsApp recebido] %s (%s):\n%s\n\n", sup.Name, from, msg)
	if err := a.qStore.UpdateBySupplier(sup.ID, msg); err != nil {
		fmt.Printf("[erro] ao registrar resposta de %s: %v\n", sup.Name, err)
	}
}

// Quote orchestrates the full buy quote flow.
func (a *Agent) Quote(ctx context.Context, description string, urgent bool) error {
	timeout := a.cfg.QuoteTimeout
	if urgent {
		timeout = 5 * time.Minute
	}

	fmt.Printf("Analisando pedido: %q\n\n", description)

	// Step 1: Parse items
	req, err := a.qManager.ParseRequest(ctx, description)
	if err != nil {
		return fmt.Errorf("analisar pedido: %w", err)
	}
	req.Urgent = urgent
	req.Timeout = timeout

	fmt.Printf("Itens identificados:\n")
	for _, it := range req.Items {
		note := ""
		if it.Note != "" {
			note = " (" + it.Note + ")"
		}
		fmt.Printf("  • %.0f %s de %s%s\n", it.Qty, it.Unit, it.Name, note)
	}
	fmt.Println()

	// Step 2: Find matching suppliers
	itemNames := make([]string, len(req.Items))
	for i, it := range req.Items {
		itemNames[i] = it.Name
	}

	sups, err := a.matcher.Match(ctx, itemNames, a.cfg.City)
	if err != nil {
		return fmt.Errorf("buscar fornecedores: %w", err)
	}

	if len(sups) == 0 {
		fmt.Println("Nenhum fornecedor encontrado para esses itens.")
		fmt.Println("Dica: cadastre fornecedores com 'comprador suppliers add'")
		return nil
	}

	fmt.Printf("Enviando cotação para %d fornecedor(es):\n", len(sups))
	for _, s := range sups {
		fmt.Printf("  • %s (%s) — %s\n", s.Supplier.Name, s.Supplier.City, s.Reason)
	}
	fmt.Println()

	// Step 3: Send quotes
	supList := make([]suppliers.Supplier, len(sups))
	for i, s := range sups {
		supList[i] = s.Supplier
	}
	if err := a.qManager.SendQuotes(ctx, req, supList); err != nil {
		return fmt.Errorf("enviar cotações: %w", err)
	}

	if a.cfg.DryRun {
		fmt.Printf("\n[dry-run] Aguardaria %.0f minutos por respostas.\n", timeout.Minutes())
		fmt.Println("Em produção, o agente fica ouvindo o WhatsApp e consolida as respostas automaticamente.")
		return nil
	}

	// Step 4: Wait for responses (handler updates DB on arrival)
	fmt.Printf("Aguardando respostas (timeout: %.0f min)...\n", timeout.Minutes())
	fmt.Println("Pressione Ctrl+C para encerrar e ver cotações parciais.\n")

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		quotes, err := a.qStore.PendingByRequest(req.ID)
		if err != nil {
			return err
		}

		received := 0
		for _, q := range quotes {
			if q.Status == "received" {
				received++
			}
		}

		fmt.Printf("\r%d/%d respostas recebidas...", received, len(sups))
		if received == len(sups) {
			fmt.Println()
			break
		}
		time.Sleep(10 * time.Second)
	}
	fmt.Println()

	// Step 5: Compare quotes
	quotes, _ := a.qStore.PendingByRequest(req.ID)
	var received []suppliers.Quote
	for _, q := range quotes {
		if q.Status == "received" {
			received = append(received, q)
		}
	}

	if len(received) == 0 {
		fmt.Println("Nenhuma resposta recebida no período.")
		a.notify(fmt.Sprintf(
			"🛒 Cotação: %q\n\nNenhuma resposta recebida em %.0f minutos.\nFornecedores contactados: %d",
			description, timeout.Minutes(), len(sups),
		))
		return nil
	}

	comparison, err := a.qManager.CompareQuotes(ctx, req, received)
	if err != nil {
		return fmt.Errorf("comparar cotações: %w", err)
	}

	fmt.Println("\n=== Comparação de Cotações ===")
	fmt.Println(comparison.Table)
	fmt.Printf("\nRecomendação: %s\n", comparison.Recommendation)
	fmt.Printf("Melhor fornecedor: %s | Total estimado: R$ %.2f\n", comparison.BestSupplier, comparison.TotalPrice)

	// Step 6: Notify owner
	a.notify(fmt.Sprintf(
		"🛒 Cotação: %q\n\n%d/%d fornecedores responderam.\n\n%s\n\nMelhor fornecedor: %s\nTotal estimado: R$ %.2f\n\nRecomendação: %s",
		description, len(received), len(sups), comparison.Table, comparison.BestSupplier, comparison.TotalPrice, comparison.Recommendation,
	))

	// Step 7: Save to memory
	_ = a.memStore.Save(memory.PurchaseRecord{
		Description:    description,
		Items:          itemNames,
		ChosenSupplier: comparison.BestSupplier,
		TotalPrice:     comparison.TotalPrice,
	})

	return nil
}

// notify sends a WhatsApp message to the owner if configured.
func (a *Agent) notify(msg string) {
	if a.cfg.OwnerPhone == "" || a.cfg.DryRun {
		return
	}
	if err := a.sender.Send(a.cfg.OwnerPhone, msg); err != nil {
		fmt.Printf("[notify] erro ao notificar dono: %v\n", err)
	} else {
		fmt.Printf("[notify] resumo enviado para %s\n", a.cfg.OwnerPhone)
	}
}

// History shows recent purchases.
func (a *Agent) History(ctx context.Context, n int) error {
	out, err := a.memStore.Format(n)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

// RepeatLast repeats the last purchase.
func (a *Agent) RepeatLast(ctx context.Context) error {
	last, err := a.memStore.Last()
	if err != nil {
		return err
	}
	if last == nil {
		fmt.Println("Nenhuma compra anterior para repetir.")
		return nil
	}
	fmt.Printf("Repetindo compra: %s\n", last.Description)
	return a.Quote(ctx, last.Description, false)
}

// AddSupplier adds a new supplier.
func (a *Agent) AddSupplier(sup suppliers.Supplier) error {
	id, err := a.supStore.Add(sup)
	if err != nil {
		return fmt.Errorf("adicionar fornecedor: %w", err)
	}
	fmt.Printf("Fornecedor adicionado: %s (ID: %s)\n", sup.Name, id)
	return nil
}

// ListSuppliers lists all active suppliers.
func (a *Agent) ListSuppliers() error {
	sups, err := a.supStore.List()
	if err != nil {
		return err
	}
	if len(sups) == 0 {
		fmt.Println("Nenhum fornecedor cadastrado.")
		return nil
	}
	fmt.Printf("%-30s %-15s %-15s %s\n", "Nome", "Cidade", "Telefone", "Categorias")
	fmt.Println("---")
	for _, s := range sups {
		cats := ""
		for i, c := range s.Categories {
			if i > 0 {
				cats += ", "
			}
			cats += c
		}
		fmt.Printf("%-30s %-15s %-15s %s\n", s.Name, s.City, s.Phone, cats)
	}
	return nil
}
