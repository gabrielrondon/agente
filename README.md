# Agente de Compras e Gestão Patrimonial

Dois agentes Go complementares para automação de compras locais e gestão de ativos.

## Agentes

### Comprador
Execução: cotação via WhatsApp com fornecedores locais, comparação de preços, coordenação de compra.

### Patrimonial
Estratégia: rastreia ativos (casa, carro, eletros), prevê necessidades, aciona o Comprador automaticamente.

## Requisitos

- Go 1.23+
- `ANTHROPIC_API_KEY` válida (Claude API)

## Setup

```bash
cp .env.example .env
# Edite .env e adicione sua ANTHROPIC_API_KEY

go mod tidy
go build ./cmd/comprador/
go build ./cmd/patrimonial/
```

## Uso — Comprador

```bash
# Solicitar cotação (modo dry-run por padrão)
./comprador quote "geladeira brastemp 400l"
./comprador quote --urgent "cabo HDMI 2m"

# Gerenciar fornecedores
./comprador suppliers add
./comprador suppliers list

# Histórico
./comprador history
./comprador repeat   # repete última compra
```

## Uso — Patrimonial

```bash
# Adicionar ativo (wizard)
./patrimonial add

# Listar ativos
./patrimonial assets list

# Dashboard com scores de risco
./patrimonial status

# Apenas alertas
./patrimonial alerts

# Registrar manutenção
./patrimonial maintenance add <asset-id>

# Acionar Comprador para um ativo
./patrimonial buy <asset-id>
```

## Arquitetura

```
agente/
├── cmd/comprador/        # CLI do Comprador
├── cmd/patrimonial/      # CLI do Patrimonial
├── comprador/            # lógica do agente comprador
│   ├── suppliers/        # fornecedores (store + matcher)
│   ├── memory/           # histórico de compras
│   ├── quote.go          # fluxo de cotação
│   └── agent.go          # orquestrador
├── patrimonial/          # lógica do agente patrimonial
│   ├── assets/           # ativos (store + predictor)
│   ├── triggers/         # integração com Comprador
│   └── agent.go          # orquestrador
└── internal/
    ├── db/               # SQLite setup
    ├── claude/           # wrapper Anthropic SDK
    └── whatsapp/         # interface + mock sender
```

## WhatsApp

Em desenvolvimento, use `--dry-run` (padrão) — as mensagens são impressas no terminal.

Para conectar WhatsApp real, descomente o código em `internal/whatsapp/session.go`
e implemente `RealSender` com `whatsmeow`. Nenhuma lógica de negócio muda.

## Banco de Dados

SQLite em `data/comprador.db` e `data/patrimonial.db`.
Criado automaticamente na primeira execução.

## Stack

| Tecnologia | Uso |
|-----------|-----|
| Go 1.23 | linguagem principal |
| `anthropic-sdk-go` | Claude API com tool use |
| `modernc.org/sqlite` | SQLite sem CGO |
| `cobra` + `viper` | CLI e configuração |
| `whatsmeow` (futuro) | WhatsApp Web protocol |
