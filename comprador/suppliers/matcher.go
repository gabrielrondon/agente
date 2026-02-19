package suppliers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/user/agente/internal/claude"
)

// Matcher uses Claude to find the best suppliers for a list of items.
type Matcher struct {
	claude *claude.Client
	store  *Store
}

// NewMatcher creates a Matcher.
func NewMatcher(claude *claude.Client, store *Store) *Matcher {
	return &Matcher{claude: claude, store: store}
}

// MatchResult holds a supplier matched to items.
type MatchResult struct {
	Supplier   Supplier
	Categories []string
	Reason     string
}

// Match asks Claude to map items to appropriate suppliers from the store.
func (m *Matcher) Match(ctx context.Context, items []string, city string) ([]MatchResult, error) {
	all, err := m.store.List()
	if err != nil {
		return nil, fmt.Errorf("list suppliers: %w", err)
	}

	if len(all) == 0 {
		return nil, nil
	}

	// Serialize suppliers for Claude
	type supSummary struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Categories []string `json:"categories"`
		City       string   `json:"city"`
		Rating     float64  `json:"rating"`
	}
	summaries := make([]supSummary, len(all))
	for i, s := range all {
		summaries[i] = supSummary{s.ID, s.Name, s.Categories, s.City, s.Rating}
	}
	supJSON, _ := json.Marshal(summaries)
	itemsJSON, _ := json.Marshal(items)

	tools := []claude.ToolDef{
		{
			Name:        "match_suppliers",
			Description: "Returns the list of supplier IDs best suited to provide the requested items",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"matches": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"supplier_id": map[string]any{"type": "string"},
								"reason":      map[string]any{"type": "string"},
							},
							"required": []string{"supplier_id", "reason"},
						},
					},
				},
				"required": []string{"matches"},
			},
		},
	}

	prompt := fmt.Sprintf(
		"Você é um assistente de compras. Dada a lista de itens a comprar e os fornecedores disponíveis, "+
			"selecione quais fornecedores devem receber pedido de cotação.\n\n"+
			"Itens: %s\n\nFornecedores disponíveis: %s\n\nCidade alvo: %s\n\n"+
			"Use a ferramenta match_suppliers para retornar os fornecedores mais adequados. "+
			"Prefira fornecedores na mesma cidade. Selecione todos que possam fornecer ao menos um item.",
		itemsJSON, supJSON, city,
	)

	var matchedIDs []struct {
		SupplierID string `json:"supplier_id"`
		Reason     string `json:"reason"`
	}

	_, err = m.claude.ChatWithTools(ctx, claude.ChatRequest{
		System: "Você é um especialista em compras locais.",
		User:   prompt,
		Tools:  tools,
	}, func(name string, input json.RawMessage) (string, error) {
		if name != "match_suppliers" {
			return "", fmt.Errorf("unknown tool: %s", name)
		}
		var result struct {
			Matches []struct {
				SupplierID string `json:"supplier_id"`
				Reason     string `json:"reason"`
			} `json:"matches"`
		}
		if err := json.Unmarshal(input, &result); err != nil {
			return "", err
		}
		matchedIDs = result.Matches
		return "ok", nil
	})
	if err != nil {
		return nil, fmt.Errorf("claude match: %w", err)
	}

	// Build index for fast lookup
	supIndex := make(map[string]Supplier, len(all))
	for _, s := range all {
		supIndex[s.ID] = s
	}

	seen := make(map[string]bool)
	var results []MatchResult
	for _, m := range matchedIDs {
		if seen[m.SupplierID] {
			continue
		}
		sup, ok := supIndex[m.SupplierID]
		if !ok {
			continue
		}
		seen[m.SupplierID] = true
		results = append(results, MatchResult{
			Supplier: sup,
			Reason:   m.Reason,
		})
	}
	return results, nil
}
