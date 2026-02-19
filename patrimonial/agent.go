package patrimonial

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/user/agente/internal/claude"
	"github.com/user/agente/patrimonial/assets"
	"github.com/user/agente/patrimonial/triggers"
)

// Agent is the main Patrimonial orchestrator.
type Agent struct {
	claude    *claude.Client
	store     *assets.Store
	predictor *assets.Predictor
	trigger   *triggers.CompradorTrigger
}

// New creates a new Patrimonial agent.
func New(db *sql.DB, cl *claude.Client) *Agent {
	store := assets.NewStore(db)
	predictor := assets.NewPredictor(cl, store)
	trigger := triggers.NewCompradorTrigger()

	return &Agent{
		claude:    cl,
		store:     store,
		predictor: predictor,
		trigger:   trigger,
	}
}

// AddAsset adds a new asset interactively.
func (a *Agent) AddAsset(asset assets.Asset) error {
	id, err := a.store.Add(asset)
	if err != nil {
		return fmt.Errorf("adicionar ativo: %w", err)
	}
	fmt.Printf("Ativo adicionado: %s (ID: %s)\n", asset.Name, id)
	return nil
}

// ListAssets lists all tracked assets.
func (a *Agent) ListAssets() error {
	assetList, err := a.store.List()
	if err != nil {
		return err
	}
	if len(assetList) == 0 {
		fmt.Println("Nenhum ativo cadastrado. Use 'patrimonial add' para adicionar.")
		return nil
	}

	fmt.Printf("%-30s %-12s %-15s %-15s %s\n", "Nome", "Tipo", "Marca/Modelo", "Localização", "Adquirido")
	fmt.Println(strings.Repeat("-", 90))
	for _, asset := range assetList {
		acquired := "—"
		if asset.AcquiredAt != nil {
			acquired = asset.AcquiredAt.Format("01/2006")
		}
		brandModel := asset.Brand
		if asset.Model != "" {
			brandModel += " " + asset.Model
		}
		fmt.Printf("%-30s %-12s %-15s %-15s %s\n",
			asset.Name, asset.Type, brandModel, asset.Location, acquired)
	}
	return nil
}

// Status shows a dashboard with risk scores for all assets.
func (a *Agent) Status(ctx context.Context) error {
	fmt.Println("Avaliando ativos com Claude...")
	assessments, err := a.predictor.AssessAll(ctx)
	if err != nil {
		return err
	}

	if len(assessments) == 0 {
		fmt.Println("Nenhum ativo cadastrado.")
		return nil
	}

	fmt.Println("\n=== Dashboard Patrimonial ===")
	fmt.Printf("%-30s %6s  %-40s %s\n", "Ativo", "Score", "Ação", "Próx. Manutenção")
	fmt.Println(strings.Repeat("-", 100))

	for _, as := range assessments {
		indicator := scoreIndicator(as.Score)
		nextDue := "—"
		if as.NextDue != nil {
			nextDue = as.NextDue.Format("02/01/2006")
		}
		action := truncate(as.Action, 40)
		fmt.Printf("%-30s %s %3d  %-40s %s\n",
			as.AssetName, indicator, as.Score, action, nextDue)
	}

	fmt.Println()
	return nil
}

// Alerts shows only assets that need attention (score > 50).
func (a *Agent) Alerts(ctx context.Context) error {
	assessments, err := a.predictor.AssessAll(ctx)
	if err != nil {
		return err
	}

	var urgent []assets.RiskAssessment
	for _, as := range assessments {
		if as.Score > 50 {
			urgent = append(urgent, as)
		}
	}

	if len(urgent) == 0 {
		fmt.Println("Nenhum alerta no momento. Todos os ativos estão em ordem.")
		return nil
	}

	fmt.Printf("=== %d Alerta(s) ===\n\n", len(urgent))
	for _, as := range urgent {
		indicator := scoreIndicator(as.Score)
		fmt.Printf("%s [%d] %s\n", indicator, as.Score, as.AssetName)
		fmt.Printf("   Raciocínio: %s\n", as.Reasoning)
		fmt.Printf("   Ação: %s\n", as.Action)
		if len(as.ProcureList) > 0 {
			fmt.Printf("   Itens a comprar: %s\n", strings.Join(as.ProcureList, ", "))
		}
		fmt.Println()
	}
	return nil
}

// AddMaintenance registers a maintenance record for an asset.
func (a *Agent) AddMaintenance(assetID string, rec assets.MaintenanceRecord) error {
	asset, err := a.store.Get(assetID)
	if err != nil || asset == nil {
		return fmt.Errorf("ativo não encontrado: %s", assetID)
	}
	rec.AssetID = assetID
	if rec.DoneAt.IsZero() {
		rec.DoneAt = time.Now()
	}
	if err := a.store.AddMaintenance(rec); err != nil {
		return fmt.Errorf("registrar manutenção: %w", err)
	}
	fmt.Printf("Manutenção registrada para %s\n", asset.Name)
	return nil
}

// Buy triggers the Comprador for a specific asset's needs.
func (a *Agent) Buy(ctx context.Context, assetID string) error {
	asset, err := a.store.Get(assetID)
	if err != nil || asset == nil {
		return fmt.Errorf("ativo não encontrado: %s", assetID)
	}

	hist, _ := a.store.MaintenanceHistory(assetID)
	assessment, err := a.predictor.AssessAsset(ctx, *asset, hist)
	if err != nil {
		return err
	}

	if len(assessment.ProcureList) == 0 {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Nenhum item automático para %s. Descreva o que precisa: ", asset.Name)
		issue, _ := reader.ReadString('\n')
		issue = strings.TrimSpace(issue)

		items, err := a.predictor.ProcurementList(ctx, *asset, issue)
		if err != nil {
			return err
		}
		assessment.ProcureList = items
	}

	_, err = a.trigger.Buy(asset.Name, assessment.ProcureList)
	return err
}

func scoreIndicator(score int) string {
	switch {
	case score >= 80:
		return "!!"
	case score >= 50:
		return " !"
	default:
		return "  "
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
