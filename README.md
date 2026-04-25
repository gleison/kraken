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

A conexão com a LLM usa o protocolo **Chat Completions da OpenAI**, que é o de
facto implementado pela maioria das ferramentas (OpenAI, Azure OpenAI, Groq,
Together, OpenRouter, Ollama, vLLM, LM Studio...). Basta apontar `OPENAI_BASE_URL`
pro endpoint desejado.

```bash
# OpenAI
export OPENAI_API_KEY=sk-...
go run ./cmd/kraken

# Ollama local
export OPENAI_API_KEY=ollama
export OPENAI_BASE_URL=http://localhost:11434/v1
export OPENAI_MODEL=llama3.1
go run ./cmd/kraken

# modo demo (sem API key → LLM mock determinístico)
go run ./cmd/kraken
```

Variáveis:

| Variável          | Default                       | Descrição                          |
|-------------------|-------------------------------|------------------------------------|
| `OPENAI_API_KEY`     | —                             | Chave de API. Se vazio, usa mock.   |
| `OPENAI_BASE_URL`    | `https://api.openai.com/v1`   | Base do endpoint compatível.        |
| `OPENAI_MODEL`       | `gpt-4o-mini`                 | Nome do modelo.                     |
| `OPENAI_TIMEOUT`     | `600`                         | Timeout por requisição (segundos).  |
| `OPENAI_MAX_TOKENS`  | `4096`                        | Tokens máximos por resposta.        |

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
