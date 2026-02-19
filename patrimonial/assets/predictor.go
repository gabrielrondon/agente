package assets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/user/agente/internal/claude"
)

// RiskAssessment is Claude's evaluation of an asset's risk.
type RiskAssessment struct {
	AssetID     string
	AssetName   string
	Score       int    // 0-100 (higher = more urgent attention)
	Reasoning   string
	Action      string // what to do
	NextDue     *time.Time
	ProcureList []string // items to buy
}

// Predictor uses Claude to assess asset risks and predict maintenance.
type Predictor struct {
	claude *claude.Client
	store  *Store
}

// NewPredictor creates a Predictor.
func NewPredictor(claude *claude.Client, store *Store) *Predictor {
	return &Predictor{claude: claude, store: store}
}

// AssessAll evaluates all assets and returns sorted risk assessments.
func (p *Predictor) AssessAll(ctx context.Context) ([]RiskAssessment, error) {
	assetList, err := p.store.List()
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}

	var assessments []RiskAssessment
	for _, a := range assetList {
		hist, err := p.store.MaintenanceHistory(a.ID)
		if err != nil {
			return nil, fmt.Errorf("history for %s: %w", a.Name, err)
		}

		assessment, err := p.AssessAsset(ctx, a, hist)
		if err != nil {
			return nil, fmt.Errorf("assess %s: %w", a.Name, err)
		}
		assessments = append(assessments, *assessment)
	}

	// Sort by score descending (most urgent first)
	for i := 0; i < len(assessments)-1; i++ {
		for j := i + 1; j < len(assessments); j++ {
			if assessments[j].Score > assessments[i].Score {
				assessments[i], assessments[j] = assessments[j], assessments[i]
			}
		}
	}

	return assessments, nil
}

// AssessAsset evaluates a single asset and returns a risk assessment.
func (p *Predictor) AssessAsset(ctx context.Context, a Asset, hist []MaintenanceRecord) (*RiskAssessment, error) {
	tools := []claude.ToolDef{
		{
			Name:        "assess_asset_risk",
			Description: "Avalia o risco de um ativo e prevê necessidades de manutenção",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"score": map[string]any{
						"type":        "integer",
						"description": "Score de risco de 0 a 100 (100 = atenção urgente)",
					},
					"reasoning": map[string]any{
						"type": "string",
					},
					"recommended_action": map[string]any{
						"type": "string",
					},
					"next_maintenance_date": map[string]any{
						"type":        "string",
						"description": "Data prevista para próxima manutenção (YYYY-MM-DD), se aplicável",
					},
					"procurement_list": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
						"description": "Lista de itens/serviços a comprar, se houver",
					},
				},
				"required": []string{"score", "reasoning", "recommended_action"},
			},
		},
	}

	assetJSON, _ := json.Marshal(a)
	histJSON, _ := json.Marshal(hist)
	today := time.Now().Format("2006-01-02")

	prompt := fmt.Sprintf(
		"Hoje é %s. Avalie o ativo:\n%s\n\nHistórico de manutenção:\n%s\n\n"+
			"Considere: idade do ativo, tempo desde última manutenção, tipo de ativo, "+
			"padrões de falha comuns. Use assess_asset_risk.",
		today, assetJSON, histJSON,
	)

	result := &RiskAssessment{
		AssetID:   a.ID,
		AssetName: a.Name,
	}

	_, err := p.claude.ChatWithTools(ctx, claude.ChatRequest{
		System: "Você é um especialista em gestão de ativos domésticos e manutenção preventiva.",
		User:   prompt,
		Tools:  tools,
	}, func(name string, input json.RawMessage) (string, error) {
		var r struct {
			Score               int      `json:"score"`
			Reasoning           string   `json:"reasoning"`
			RecommendedAction   string   `json:"recommended_action"`
			NextMaintenanceDate string   `json:"next_maintenance_date"`
			ProcurementList     []string `json:"procurement_list"`
		}
		if err := json.Unmarshal(input, &r); err != nil {
			return "", err
		}
		result.Score = r.Score
		result.Reasoning = r.Reasoning
		result.Action = r.RecommendedAction
		result.ProcureList = r.ProcurementList
		if r.NextMaintenanceDate != "" {
			t, _ := time.Parse("2006-01-02", r.NextMaintenanceDate)
			result.NextDue = &t
		}
		return "ok", nil
	})
	if err != nil {
		return nil, fmt.Errorf("claude assess: %w", err)
	}

	return result, nil
}

// ProcurementList generates a shopping list for a specific asset issue.
func (p *Predictor) ProcurementList(ctx context.Context, asset Asset, issue string) ([]string, error) {
	tools := []claude.ToolDef{
		{
			Name:        "generate_procurement_list",
			Description: "Gera lista de itens e serviços a comprar para resolver um problema no ativo",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
				"required": []string{"items"},
			},
		},
	}

	prompt := fmt.Sprintf(
		"Ativo: %s (%s %s)\nProblema/necessidade: %s\n\n"+
			"Liste todos os materiais e serviços necessários para resolver isso.",
		asset.Name, asset.Brand, asset.Model, issue,
	)

	var items []string
	_, err := p.claude.ChatWithTools(ctx, claude.ChatRequest{
		System: "Você é um especialista em manutenção residencial e de veículos.",
		User:   prompt,
		Tools:  tools,
	}, func(name string, input json.RawMessage) (string, error) {
		var r struct {
			Items []string `json:"items"`
		}
		if err := json.Unmarshal(input, &r); err != nil {
			return "", err
		}
		items = r.Items
		return "ok", nil
	})
	if err != nil {
		return nil, err
	}

	return items, nil
}
