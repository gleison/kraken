# kraken

Orquestrador para usar LLMs de forma mais assertiva.

Decompõe uma tarefa complexa em um **plano** de tarefas simples e diretas, e
executa cada tarefa com um LLM, passando o resultado das tarefas anteriores
como contexto. Vem com uma TUI (Bubble Tea) para acompanhar a execução em tempo
real.

## Arquitetura

Camadas (Clean Architecture — dependências apontam sempre pra dentro):

```
cmd/kraken            entrada, wiring
internal/tui          apresentação (Bubble Tea)
internal/orchestrator casos de uso (Planner, Executor, Orchestrator)
internal/llm          porta + adaptadores (Anthropic, Mock)
internal/domain       entidades puras (Task, Plan)
```

Fluxo:

```
Goal ──► Planner ──► Plan (tarefas simples)
                       │
                       ▼
                    Executor  ◄── contexto acumulado
                       │
                       ▼
                    Eventos  ──► TUI
```

- `llm.Client` é uma **porta**: trocar de provedor é implementar a interface.
- `Orchestrator.Run` retorna um **canal de eventos**, o que desacopla a execução
  do render.

## Uso

```bash
# com API real
export ANTHROPIC_API_KEY=sk-ant-...
go run ./cmd/kraken

# modo demo (sem API key → LLM mock determinístico)
go run ./cmd/kraken
```

Variáveis opcionais:

- `ANTHROPIC_API_KEY` — chave da API Anthropic. Sem ela, usa-se o mock.
- `KRAKEN_MODEL` — modelo Anthropic (default: `claude-opus-4-7`).

### Atalhos da TUI

- `Enter` — executa o objetivo (ou inicia nova tarefa na tela final)
- `r` — nova tarefa (após concluir)
- `q` — sair (após concluir)
- `Ctrl+C` — sair a qualquer momento

## Build

```bash
go build -o bin/kraken ./cmd/kraken
./bin/kraken
```
