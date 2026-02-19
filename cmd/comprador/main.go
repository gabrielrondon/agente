package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/user/agente/comprador"
	"github.com/user/agente/comprador/suppliers"
	"github.com/user/agente/internal/claude"
	"github.com/user/agente/internal/db"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var (
		dryRun  bool
		city    string
		dbPath  string
		timeout int
	)

	root := &cobra.Command{
		Use:   "comprador",
		Short: "Agente de compras com cotação via WhatsApp",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			viper.SetConfigName(".env")
			viper.SetConfigType("env")
			viper.AddConfigPath(".")
			viper.AutomaticEnv()
			_ = viper.ReadInConfig()
			return nil
		},
	}

	root.PersistentFlags().BoolVar(&dryRun, "dry-run", true, "Modo simulação (não envia WhatsApp real)")
	root.PersistentFlags().StringVar(&city, "city", "local", "Cidade para filtrar fornecedores")
	root.PersistentFlags().StringVar(&dbPath, "db", "data/comprador.db", "Caminho do banco SQLite")
	root.PersistentFlags().IntVar(&timeout, "timeout", 30, "Timeout de cotação em minutos")

	// Helper to build agent
	buildAgent := func(cmd *cobra.Command) (*comprador.Agent, error) {
		database, err := db.Open(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open db: %w", err)
		}

		cl, err := claude.New()
		if err != nil {
			return nil, err
		}

		cfg := comprador.Config{
			City:         city,
			QuoteTimeout: time.Duration(timeout) * time.Minute,
			DryRun:       dryRun,
		}

		return comprador.New(database, cl, cfg), nil
	}

	// quote command
	quoteCmd := &cobra.Command{
		Use:   "quote [descrição]",
		Short: "Solicita cotação de preço para um item ou lista de itens",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			urgent, _ := cmd.Flags().GetBool("urgent")
			agent, err := buildAgent(cmd)
			if err != nil {
				return err
			}
			description := strings.Join(args, " ")
			return agent.Quote(cmd.Context(), description, urgent)
		},
	}
	quoteCmd.Flags().Bool("urgent", false, "Cotação urgente (timeout 5 min)")

	// suppliers commands
	suppliersCmd := &cobra.Command{
		Use:   "suppliers",
		Short: "Gerenciar fornecedores",
	}

	suppliersAddCmd := &cobra.Command{
		Use:   "add",
		Short: "Cadastrar novo fornecedor",
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := buildAgent(cmd)
			if err != nil {
				return err
			}
			sup, err := promptSupplier()
			if err != nil {
				return err
			}
			return agent.AddSupplier(sup)
		},
	}

	suppliersListCmd := &cobra.Command{
		Use:   "list",
		Short: "Listar fornecedores ativos",
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := buildAgent(cmd)
			if err != nil {
				return err
			}
			return agent.ListSuppliers()
		},
	}

	suppliersCmd.AddCommand(suppliersAddCmd, suppliersListCmd)

	// history command
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Exibir histórico de compras",
		RunE: func(cmd *cobra.Command, args []string) error {
			n, _ := cmd.Flags().GetInt("last")
			agent, err := buildAgent(cmd)
			if err != nil {
				return err
			}
			return agent.History(cmd.Context(), n)
		},
	}
	historyCmd.Flags().Int("last", 10, "Número de compras a exibir")

	// repeat command
	repeatCmd := &cobra.Command{
		Use:   "repeat",
		Short: "Repetir a última compra",
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := buildAgent(cmd)
			if err != nil {
				return err
			}
			return agent.RepeatLast(cmd.Context())
		},
	}

	root.AddCommand(quoteCmd, suppliersCmd, historyCmd, repeatCmd)
	return root
}

func promptSupplier() (suppliers.Supplier, error) {
	reader := bufio.NewReader(os.Stdin)
	read := func(prompt string) string {
		fmt.Print(prompt)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}

	name := read("Nome do fornecedor: ")
	phone := read("Telefone (ex: 5511999999999): ")
	city := read("Cidade: ")
	catsRaw := read("Categorias (vírgula): ")

	var cats []string
	for _, c := range strings.Split(catsRaw, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			cats = append(cats, c)
		}
	}

	return suppliers.Supplier{
		Name:       name,
		Phone:      phone,
		City:       city,
		Categories: cats,
		Rating:     5.0,
		Active:     true,
	}, nil
}
