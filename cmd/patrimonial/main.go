package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/user/agente/internal/claude"
	"github.com/user/agente/internal/db"
	"github.com/user/agente/patrimonial"
	"github.com/user/agente/patrimonial/assets"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var dbPath string

	root := &cobra.Command{
		Use:   "patrimonial",
		Short: "Agente de gestão patrimonial — rastreia ativos e prevê manutenção",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			viper.SetConfigName(".env")
			viper.SetConfigType("env")
			viper.AddConfigPath(".")
			viper.AutomaticEnv()
			_ = viper.ReadInConfig()
			return nil
		},
	}

	root.PersistentFlags().StringVar(&dbPath, "db", "data/patrimonial.db", "Caminho do banco SQLite")

	buildAgent := func() (*patrimonial.Agent, error) {
		database, err := db.Open(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open db: %w", err)
		}
		cl, err := claude.New()
		if err != nil {
			return nil, err
		}
		return patrimonial.New(database, cl), nil
	}

	// add command
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Adicionar novo ativo (wizard interativo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := buildAgent()
			if err != nil {
				return err
			}
			asset, err := promptAsset()
			if err != nil {
				return err
			}
			return agent.AddAsset(asset)
		},
	}

	// assets list command
	assetsCmd := &cobra.Command{
		Use:   "assets",
		Short: "Gerenciar ativos",
	}

	assetsListCmd := &cobra.Command{
		Use:   "list",
		Short: "Listar todos os ativos",
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := buildAgent()
			if err != nil {
				return err
			}
			return agent.ListAssets()
		},
	}
	assetsCmd.AddCommand(assetsListCmd)

	// status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Dashboard com scores de risco de todos os ativos",
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := buildAgent()
			if err != nil {
				return err
			}
			return agent.Status(cmd.Context())
		},
	}

	// alerts command
	alertsCmd := &cobra.Command{
		Use:   "alerts",
		Short: "Mostrar apenas ativos que precisam de atenção",
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := buildAgent()
			if err != nil {
				return err
			}
			return agent.Alerts(cmd.Context())
		},
	}

	// maintenance add command
	maintenanceCmd := &cobra.Command{
		Use:   "maintenance",
		Short: "Gerenciar manutenções",
	}

	maintenanceAddCmd := &cobra.Command{
		Use:   "add <asset-id>",
		Short: "Registrar manutenção realizada",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := buildAgent()
			if err != nil {
				return err
			}
			rec, err := promptMaintenance()
			if err != nil {
				return err
			}
			return agent.AddMaintenance(args[0], rec)
		},
	}
	maintenanceCmd.AddCommand(maintenanceAddCmd)

	// buy command
	buyCmd := &cobra.Command{
		Use:   "buy <asset-id>",
		Short: "Acionar Comprador para comprar itens necessários para um ativo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			issue, _ := cmd.Flags().GetString("issue")
			agent, err := buildAgent()
			if err != nil {
				return err
			}
			return agent.Buy(cmd.Context(), args[0], issue)
		},
	}
	buyCmd.Flags().String("issue", "", "Descreva o problema ou itens necessários (evita prompt interativo)")

	root.AddCommand(addCmd, assetsCmd, statusCmd, alertsCmd, maintenanceCmd, buyCmd)
	return root
}

func promptAsset() (assets.Asset, error) {
	reader := bufio.NewReader(os.Stdin)
	read := func(prompt string) string {
		fmt.Print(prompt)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}

	name := read("Nome do ativo: ")
	assetType := read("Tipo (house/car/appliance/other): ")
	brand := read("Marca: ")
	model := read("Modelo: ")
	location := read("Localização: ")
	acquiredStr := read("Data de aquisição (MM/AAAA, ou vazio): ")
	notes := read("Observações: ")

	var acquiredAt *time.Time
	if acquiredStr != "" {
		t, err := time.Parse("01/2006", acquiredStr)
		if err == nil {
			acquiredAt = &t
		}
	}

	return assets.Asset{
		Name:       name,
		Type:       assetType,
		Brand:      brand,
		Model:      model,
		Location:   location,
		AcquiredAt: acquiredAt,
		Notes:      notes,
		Metadata:   make(map[string]any),
	}, nil
}

func promptMaintenance() (assets.MaintenanceRecord, error) {
	reader := bufio.NewReader(os.Stdin)
	read := func(prompt string) string {
		fmt.Print(prompt)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}

	description := read("Descrição da manutenção: ")
	supplier := read("Fornecedor/prestador: ")
	costStr := read("Custo (ex: 150.00): ")
	nextDueStr := read("Próxima manutenção (MM/AAAA, ou vazio): ")

	var cost float64
	fmt.Sscanf(costStr, "%f", &cost)

	var nextDue *time.Time
	if nextDueStr != "" {
		t, err := time.Parse("01/2006", nextDueStr)
		if err == nil {
			nextDue = &t
		}
	}

	return assets.MaintenanceRecord{
		Description: description,
		Supplier:    supplier,
		Cost:        cost,
		DoneAt:      time.Now(),
		NextDue:     nextDue,
	}, nil
}
