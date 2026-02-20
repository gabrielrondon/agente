# Agente — Estado do Projeto

## O que é
Dois agentes em Go para automação de compras locais via WhatsApp:
- **Comprador** (`cmd/comprador/`) — cotação via WhatsApp com fornecedores locais
- **Patrimonial** (`cmd/patrimonial/`) — rastreia ativos (casa, carro, eletros), prevê manutenção, aciona o Comprador

Stack: Go, whatsmeow (WhatsApp Web), DeepSeek v3 via OpenRouter, SQLite (modernc.org/sqlite).

## Onde paramos (fev/2026)

### Implementado e funcionando
- [x] Comprador: cotação WhatsApp real (envia + aguarda respostas + compara)
- [x] Comprador: `--dry-run`, `--urgent`, `--owner`, `--yes` flags
- [x] Comprador: confirmação interativa dos itens antes de enviar (loop com correção via Claude)
- [x] Comprador: notificação ao dono via WhatsApp quando cotação é enviada
- [x] Comprador: notificação ao dono via WhatsApp quando comparação fica pronta
- [x] Comprador: deduplicação de fornecedores no matcher
- [x] Patrimonial: `add`, `assets list`, `status`, `alerts`, `buy --issue`
- [x] Patrimonial: specs técnicas automáticas usando marca/modelo do ativo
- [x] WhatsApp: lock file com PID para evitar sessões simultâneas (previne restrição de conta)
- [x] WhatsApp: `bin/qr` helper para parear sem subir o agente completo

### Próximas melhorias planejadas (em ordem de prioridade)
1. **Hash de contexto nas mensagens** — incluir código `COT-xxxxx` nas mensagens para fornecedores consultarem contexto completo
2. **Agente do fornecedor (B2B)** — produto separado: bot WhatsApp que responde cotações automaticamente
3. **Scheduler** — `patrimonial status` automático diário, alertas por WhatsApp
4. **`patrimonial buy` → rodar comprador diretamente** (hoje só imprime o comando)
5. **Follow-up automático** — lembrete para fornecedores que não responderam

## Situação do WhatsApp
- Conta pessoal (`+351969210117`) ficou restrita 5h por múltiplos QR scans em sequência
- **Usar número dedicado para o bot** — não o número pessoal
- Após restrição levantar: `./bin/qr` para parear novamente
- Lock file `data/whatsapp.lock` impede sessões simultâneas daqui pra frente

## Env vars necessárias (.env)
```
OPENROUTER_API_KEY=...
OWNER_PHONE=351969210117   # recebe notificações WhatsApp
```

## Comandos úteis
```bash
# Parear WhatsApp
./bin/qr

# Cotação interativa (pede confirmação dos itens)
./bin/comprador quote "pastilha de freio VW Gol G6"

# Cotação sem confirmação (background/scripts)
./bin/comprador quote --yes "pastilha de freio VW Gol G6"

# Patrimonial
./bin/patrimonial status
./bin/patrimonial alerts
./bin/patrimonial buy <asset-id> --issue "descrição do problema"

# Build tudo
go build -o bin/comprador ./cmd/comprador/
go build -o bin/patrimonial ./cmd/patrimonial/
go build -o bin/qr ./cmd/qr/
```

## Fornecedores no banco
32 fornecedores reais de Campo Grande/MS (seed em `data/comprador.db`).
Categorias: autopeças, supermercado, construção, elétrica, hidráulica, etc.

## Ativos cadastrados (data/patrimonial.db)
- Carro — VW Gol G6 (2018)
- Casa (2015)
- Geladeira — Brastemp BRM44HB (2019)
- Geladeira Brastemp — duplicata, remover (BRF400, 04/2019)
