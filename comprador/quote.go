package comprador

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/user/agente/comprador/suppliers"
	"github.com/user/agente/internal/claude"
	"github.com/user/agente/internal/whatsapp"
)

// QuoteRequest describes what the user wants to buy.
type QuoteRequest struct {
	ID          string
	Description string
	Items       []ParsedItem
	Urgent      bool
	Timeout     time.Duration
}

// ParsedItem is an item extracted from the user's description.
type ParsedItem struct {
	Name string  `json:"name"`
	Qty  float64 `json:"qty"`
	Unit string  `json:"unit"`
	Note string  `json:"note,omitempty"`
}

// QuoteComparison is the final analysis from Claude.
type QuoteComparison struct {
	Recommendation string
	BestSupplier   string
	TotalPrice     float64
	Table          string // text table for display
}

// QuoteManager orchestrates the quoting flow.
type QuoteManager struct {
	claude     *claude.Client
	sender     whatsapp.MessageSender
	supStore   *suppliers.Store
	quoteStore *suppliers.QuoteStore
}

// NewQuoteManager creates a QuoteManager.
func NewQuoteManager(
	cl *claude.Client,
	sender whatsapp.MessageSender,
	supStore *suppliers.Store,
	quoteStore *suppliers.QuoteStore,
) *QuoteManager {
	return &QuoteManager{
		claude:     cl,
		sender:     sender,
		supStore:   supStore,
		quoteStore: quoteStore,
	}
}

// ParseRequest uses Claude to extract structured items from free-form text.
func (qm *QuoteManager) ParseRequest(ctx context.Context, description string) (*QuoteRequest, error) {
	tools := []claude.ToolDef{
		{
			Name:        "parse_purchase_request",
			Description: "Extrai a lista de itens e quantidades a partir de uma descrição de compra em linguagem natural",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name": map[string]any{"type": "string", "description": "Nome do item"},
								"qty":  map[string]any{"type": "number", "description": "Quantidade"},
								"unit": map[string]any{"type": "string", "description": "Unidade (kg, litro, unid, m², etc)"},
								"note": map[string]any{"type": "string", "description": "Especificação adicional, se houver"},
							},
							"required": []string{"name", "qty", "unit"},
						},
					},
				},
				"required": []string{"items"},
			},
		},
	}

	var items []ParsedItem
	_, err := qm.claude.ChatWithTools(ctx, claude.ChatRequest{
		System: "Você é um assistente de compras. Interprete pedidos de compra e extraia itens com quantidades precisas.",
		User:   fmt.Sprintf("Analise esta solicitação de compra e extraia os itens: %q", description),
		Tools:  tools,
	}, func(name string, input json.RawMessage) (string, error) {
		var result struct {
			Items []ParsedItem `json:"items"`
		}
		if err := json.Unmarshal(input, &result); err != nil {
			return "", err
		}
		items = result.Items
		return "ok", nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse request: %w", err)
	}

	return &QuoteRequest{
		ID:          uuid.New().String(),
		Description: description,
		Items:       items,
	}, nil
}

// SendQuotes sends a quote request message to each supplier.
func (qm *QuoteManager) SendQuotes(ctx context.Context, req *QuoteRequest, sups []suppliers.Supplier) error {
	for _, sup := range sups {
		msg, err := qm.composeMessage(ctx, req, sup)
		if err != nil {
			return fmt.Errorf("compose message for %s: %w", sup.Name, err)
		}

		if err := qm.sender.Send(sup.Phone, msg); err != nil {
			return fmt.Errorf("send to %s: %w", sup.Name, err)
		}

		// Record the pending quote
		itemsRaw := make([]suppliers.QuoteItem, len(req.Items))
		for i, it := range req.Items {
			itemsRaw[i] = suppliers.QuoteItem{Name: it.Name, Qty: it.Qty, Unit: it.Unit}
		}
		if err := qm.quoteStore.CreateQuote(suppliers.Quote{
			ID:         uuid.New().String(),
			RequestID:  req.ID,
			SupplierID: sup.ID,
			Items:      itemsRaw,
			CreatedAt:  time.Now(),
		}); err != nil {
			return fmt.Errorf("save quote: %w", err)
		}
	}
	return nil
}

func (qm *QuoteManager) composeMessage(ctx context.Context, req *QuoteRequest, sup suppliers.Supplier) (string, error) {
	tools := []claude.ToolDef{
		{
			Name:        "compose_quote_message",
			Description: "Compõe a mensagem de WhatsApp a enviar ao fornecedor pedindo cotação",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]any{"type": "string", "description": "Mensagem completa de WhatsApp"},
				},
				"required": []string{"message"},
			},
		},
	}

	itemsJSON, _ := json.Marshal(req.Items)
	prompt := fmt.Sprintf(
		"Crie uma mensagem de WhatsApp profissional e amigável para o fornecedor %q pedindo cotação dos seguintes itens:\n%s\n"+
			"A mensagem deve ser clara, incluir os itens e quantidades, e pedir preço unitário e prazo de entrega.",
		sup.Name, itemsJSON,
	)

	var message string
	_, err := qm.claude.ChatWithTools(ctx, claude.ChatRequest{
		System: "Você cria mensagens de WhatsApp para cotação de preços com fornecedores locais. Seja direto e profissional.",
		User:   prompt,
		Tools:  tools,
	}, func(name string, input json.RawMessage) (string, error) {
		var result struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(input, &result); err != nil {
			return "", err
		}
		message = result.Message
		return "ok", nil
	})
	if err != nil {
		return "", err
	}

	if message == "" {
		// Fallback: build a simple message
		var lines []string
		lines = append(lines, fmt.Sprintf("Olá %s! Preciso de uma cotação para os seguintes itens:", sup.Name))
		for _, it := range req.Items {
			lines = append(lines, fmt.Sprintf("- %s: %.0f %s", it.Name, it.Qty, it.Unit))
		}
		lines = append(lines, "\nPor favor, informe o preço unitário e prazo de entrega. Obrigado!")
		message = strings.Join(lines, "\n")
	}

	return message, nil
}

// CompareQuotes uses Claude to analyze received quotes and recommend the best.
func (qm *QuoteManager) CompareQuotes(ctx context.Context, req *QuoteRequest, quotes []suppliers.Quote) (*QuoteComparison, error) {
	if len(quotes) == 0 {
		return &QuoteComparison{Recommendation: "Nenhuma cotação recebida."}, nil
	}

	tools := []claude.ToolDef{
		{
			Name:        "compare_quotes",
			Description: "Compara as cotações recebidas e recomenda a melhor opção",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"recommendation":  map[string]any{"type": "string"},
					"best_supplier":   map[string]any{"type": "string"},
					"total_price":     map[string]any{"type": "number"},
					"comparison_table": map[string]any{"type": "string", "description": "Tabela texto comparando fornecedores"},
				},
				"required": []string{"recommendation", "best_supplier", "comparison_table"},
			},
		},
	}

	quotesJSON, _ := json.Marshal(quotes)
	prompt := fmt.Sprintf(
		"Analise as cotações recebidas para: %q\n\nCotações:\n%s\n\n"+
			"Use compare_quotes para recomendar a melhor opção, considerando preço, prazo e qualidade.",
		req.Description, quotesJSON,
	)

	var result QuoteComparison
	_, err := qm.claude.ChatWithTools(ctx, claude.ChatRequest{
		System: "Você é um especialista em compras. Analise cotações e recomende a melhor opção custo-benefício.",
		User:   prompt,
		Tools:  tools,
	}, func(name string, input json.RawMessage) (string, error) {
		var r struct {
			Recommendation  string  `json:"recommendation"`
			BestSupplier    string  `json:"best_supplier"`
			TotalPrice      float64 `json:"total_price"`
			ComparisonTable string  `json:"comparison_table"`
		}
		if err := json.Unmarshal(input, &r); err != nil {
			return "", err
		}
		result = QuoteComparison{
			Recommendation: r.Recommendation,
			BestSupplier:   r.BestSupplier,
			TotalPrice:     r.TotalPrice,
			Table:          r.ComparisonTable,
		}
		return "ok", nil
	})
	if err != nil {
		return nil, fmt.Errorf("compare quotes: %w", err)
	}

	return &result, nil
}
