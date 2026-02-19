// cmd/seed/main.go — popula o banco com fornecedores de Campo Grande / MS
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/user/agente/comprador/suppliers"
	"github.com/user/agente/internal/db"
)

func main() {
	dbPath := "data/comprador.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	store := suppliers.NewStore(database)

	sups := []suppliers.Supplier{
		// ── Materiais de construção ─────────────────────────────────────
		{Name: "Cunha Materiais de Construção", Phone: "5567992744867", City: "Campo Grande",
			Categories: []string{"materiais_construcao"}, Rating: 5, Active: true},
		{Name: "Elos Comércio de Materiais de Construção", Phone: "556730423826", City: "Campo Grande",
			Categories: []string{"materiais_construcao"}, Rating: 5, Active: true},
		{Name: "Cimento e Ferro", Phone: "556733316505", City: "Campo Grande",
			Categories: []string{"materiais_construcao"}, Rating: 5, Active: true},
		{Name: "Rede Sertão", Phone: "556733047401", City: "Campo Grande",
			Categories: []string{"materiais_construcao", "ferramentas_ferragens", "eletrica_hidraulica"}, Rating: 5, Active: true},
		{Name: "Leroy Merlin Campo Grande", Phone: "556740205376", City: "Campo Grande",
			Categories: []string{"materiais_construcao", "ferramentas_ferragens", "eletrica_hidraulica", "eletrodomesticos"}, Rating: 5, Active: true},
		{Name: "Pinheirão Madeiras e Ferragens", Phone: "556733000000", City: "Campo Grande",
			Categories: []string{"materiais_construcao", "ferramentas_ferragens"}, Rating: 5, Active: true},

		// ── Carnes / Açougue ────────────────────────────────────────────
		{Name: "Província da Carne - Matriz", Phone: "5567996447007", City: "Campo Grande",
			Categories: []string{"carnes_acougue", "churrasco"}, Rating: 5, Active: true},
		{Name: "Província da Carne - Damha", Phone: "5567999626088", City: "Campo Grande",
			Categories: []string{"carnes_acougue", "churrasco"}, Rating: 5, Active: true},
		{Name: "Terruáh Província da Carne", Phone: "5567991226903", City: "Campo Grande",
			Categories: []string{"carnes_acougue", "churrasco"}, Rating: 5, Active: true},
		{Name: "Fazenda Churrascada", Phone: "5567998952010", City: "Campo Grande",
			Categories: []string{"carnes_acougue", "churrasco"}, Rating: 5, Active: true},

		// ── Hortifruti ──────────────────────────────────────────────────
		{Name: "Hortifruti Santa Rita - Guaicurus", Phone: "5567981611444", City: "Campo Grande",
			Categories: []string{"hortifruti", "frutas", "verduras"}, Rating: 5, Active: true},
		{Name: "Hortifruti Santa Rita - São Francisco", Phone: "5567991658850", City: "Campo Grande",
			Categories: []string{"hortifruti", "frutas", "verduras"}, Rating: 5, Active: true},
		{Name: "Florestal Hortifruti", Phone: "556733611128", City: "Campo Grande",
			Categories: []string{"hortifruti", "frutas", "verduras"}, Rating: 5, Active: true},
		{Name: "Pereira Hortifruti", Phone: "556733137300", City: "Campo Grande",
			Categories: []string{"hortifruti", "frutas", "verduras"}, Rating: 5, Active: true},

		// ── Atacado / Supermercado ──────────────────────────────────────
		{Name: "Assaí Atacadista - Aeroporto", Phone: "556733681650", City: "Campo Grande",
			Categories: []string{"supermercado_atacado", "alimentos", "limpeza", "carnes_acougue"}, Rating: 5, Active: true},
		{Name: "Assaí Atacadista - Joaquim Murtinho", Phone: "556733574550", City: "Campo Grande",
			Categories: []string{"supermercado_atacado", "alimentos", "limpeza", "carnes_acougue"}, Rating: 5, Active: true},
		{Name: "Atacadão - Costa e Silva", Phone: "556733454444", City: "Campo Grande",
			Categories: []string{"supermercado_atacado", "alimentos", "limpeza"}, Rating: 5, Active: true},
		{Name: "Atacadão - Coronel Antonino", Phone: "556733124444", City: "Campo Grande",
			Categories: []string{"supermercado_atacado", "alimentos", "limpeza"}, Rating: 5, Active: true},
		{Name: "Fort Atacadista - Cafezais", Phone: "556740097114", City: "Campo Grande",
			Categories: []string{"supermercado_atacado", "alimentos", "limpeza"}, Rating: 5, Active: true},
		{Name: "Fort Atacadista - Rua da Divisão", Phone: "556740097114", City: "Campo Grande",
			Categories: []string{"supermercado_atacado", "alimentos", "limpeza"}, Rating: 5, Active: true},

		// ── Eletrodomésticos ────────────────────────────────────────────
		{Name: "Magazine Luiza - Centro", Phone: "556733896000", City: "Campo Grande",
			Categories: []string{"eletrodomesticos", "eletronicos"}, Rating: 5, Active: true},
		{Name: "Magazine Luiza - São Francisco", Phone: "556733188700", City: "Campo Grande",
			Categories: []string{"eletrodomesticos", "eletronicos"}, Rating: 5, Active: true},
		{Name: "Magazine Luiza - Coronel Antonino", Phone: "556733582400", City: "Campo Grande",
			Categories: []string{"eletrodomesticos", "eletronicos"}, Rating: 5, Active: true},
		{Name: "Lojas Americanas Campo Grande", Phone: "556740034848", City: "Campo Grande",
			Categories: []string{"eletrodomesticos", "eletronicos", "alimentos"}, Rating: 5, Active: true},

		// ── Autopeças ───────────────────────────────────────────────────
		{Name: "Sama Autopeças", Phone: "556733458880", City: "Campo Grande",
			Categories: []string{"autopecas", "manutencao_veiculo"}, Rating: 5, Active: true},
		{Name: "Laguna Autopeças", Phone: "556733458800", City: "Campo Grande",
			Categories: []string{"autopecas", "manutencao_veiculo"}, Rating: 5, Active: true},
		{Name: "Auto Peças Paraná", Phone: "556733842456", City: "Campo Grande",
			Categories: []string{"autopecas", "manutencao_veiculo"}, Rating: 5, Active: true},
		{Name: "JBR Auto Peças", Phone: "5567992887101", City: "Campo Grande",
			Categories: []string{"autopecas", "manutencao_veiculo"}, Rating: 5, Active: true},

		// ── Elétrica / Hidráulica ───────────────────────────────────────
		{Name: "Elétrica Polo", Phone: "556733485811", City: "Campo Grande",
			Categories: []string{"eletrica_hidraulica", "materiais_construcao"}, Rating: 5, Active: true},
		{Name: "AMGL Materiais Elétricos e Hidráulicos", Phone: "556733242258", City: "Campo Grande",
			Categories: []string{"eletrica_hidraulica"}, Rating: 5, Active: true},

		// ── Ferramentas / Ferragens ─────────────────────────────────────
		{Name: "Central Máquinas e Ferramentas", Phone: "556733513311", City: "Campo Grande",
			Categories: []string{"ferramentas_ferragens"}, Rating: 5, Active: true},
		{Name: "Azulão Parafusos", Phone: "556733513000", City: "Campo Grande",
			Categories: []string{"ferramentas_ferragens", "materiais_construcao"}, Rating: 5, Active: true},
	}

	ok, skip := 0, 0
	for _, s := range sups {
		_, err := store.Add(s)
		if err != nil {
			fmt.Printf("  SKIP  %s — %v\n", s.Name, err)
			skip++
			continue
		}
		fmt.Printf("  ADD   %s\n", s.Name)
		ok++
	}

	fmt.Printf("\n%d adicionados, %d ignorados.\n", ok, skip)
}
