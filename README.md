# Kraken

Assistente de terminal para conversar com LLMs e, quando necessário, decompor
objetivos complexos em planos executáveis.

O modo padrão é **chat**, com conversa direta e ferramentas de workspace. O
modo **plan** decompõe uma tarefa complexa em passos simples, executando cada
passo com o resultado dos anteriores como contexto. A TUI usa Bubble Tea.

## Arquitetura

Camadas (Clean Architecture — dependências apontam sempre pra dentro):

```
cmd/kraken            entrada, wiring
internal/tui          apresentação (Bubble Tea)
internal/orchestrator casos de uso (Planner, Executor, Orchestrator)
internal/llm          porta + adaptador OpenAI-compatible
internal/domain       entidades puras (Task, Plan)
internal/tools        ferramentas confinadas ao workspace
```

Fluxo:

```
Mensagem ──► Chat ──────────────────────┐
                                       ▼
Objetivo ──► Planner ──► Plan ──► Executor
                                       │
                                       ▼
                                    Eventos ──► TUI
```

- `llm.Client` é uma **porta**: trocar de provedor é implementar a interface.
- `orchestrator.Planning` e `orchestrator.Execution` separam as estratégias de
  planejamento e execução. `NewWithPorts` recebe essas dependências; `New`
  mantém a composição padrão com LLM.
- A TUI usa `orchestrator.RunService`; o executor usa `tools.Catalog`.
  Contratos ficam em arquivos separados das implementações, e a configuração
  dos componentes concretos fica na entrada da aplicação.
- Os fluxos de chat e plan retornam **canais de eventos**, desacoplando a
  execução do render.

## Uso

A conexão com a LLM usa o protocolo **Chat Completions da OpenAI**, que é o de
facto implementado pela maioria das ferramentas (OpenAI, Groq, Together,
OpenRouter, Ollama, vLLM, LM Studio...). Basta apontar `OPENAI_BASE_URL`
pro endpoint desejado.

É obrigatório configurar pelo menos uma das duas variáveis: `OPENAI_API_KEY`
(provedores na nuvem) ou `OPENAI_BASE_URL` (provedores locais sem chave). Sem
nenhuma das duas o `kraken` falha no startup com instrução do que falta.

```bash
# OpenAI (nuvem)
export OPENAI_API_KEY=sk-...
go run ./cmd/kraken

# iniciar diretamente no modo plan
go run ./cmd/kraken --plan

# LM Studio local (sem chave)
export OPENAI_BASE_URL=http://localhost:1234/v1
export OPENAI_MODEL=gemma-2-2b-it
go run ./cmd/kraken

# Ollama local
export OPENAI_BASE_URL=http://localhost:11434/v1
export OPENAI_MODEL=llama3.1
go run ./cmd/kraken
```

Variáveis:

| Variável             | Default                       | Descrição                                 |
|----------------------|-------------------------------|-------------------------------------------|
| `OPENAI_API_KEY`     | —                             | Chave da API. Opcional p/ provedor local. |
| `OPENAI_BASE_URL`    | `https://api.openai.com/v1`   | Base do endpoint compatível.              |
| `OPENAI_MODEL`       | `gpt-4o-mini`                 | Nome do modelo.                           |
| `OPENAI_TIMEOUT`     | `600`                         | Timeout por requisição (segundos).        |
| `OPENAI_MAX_TOKENS`  | `4096`                        | Tokens máximos por resposta.              |
| `KRAKEN_LOG`         | —                             | Caminho de arquivo p/ log de diagnóstico. |
| `KRAKEN_WORKSPACE`   | cwd                           | Pasta-raiz que as ferramentas podem ler/escrever. |
| `KRAKEN_ALLOW_WRITE` | `0`                           | `1` habilita `write_file` (ou flag `--write`).    |

### Ferramentas de arquivos

A LLM pode chamar três ferramentas confinadas ao workspace (sem escapar com
`..` ou symlinks que apontem pra fora):

- `read_file` (sempre on) — lê um arquivo UTF-8.
- `list_dir` (sempre on) — lista entradas de um diretório.
- `write_file` (off por padrão) — cria/sobrescreve atomicamente arquivos de até
  1 MiB. Habilite com `--write` ou `KRAKEN_ALLOW_WRITE=1`.

Arquivos lidos são enviados ao endpoint configurado. Use um workspace restrito
e não inclua segredos quando estiver conectado a um provedor remoto.

```bash
# leitura/listagem apenas, sem permitir escrita:
KRAKEN_WORKSPACE=$PWD go run ./cmd/kraken

# permitindo escrita também:
KRAKEN_WORKSPACE=$PWD go run ./cmd/kraken --write

# as flags equivalentes também podem ser usadas:
go run ./cmd/kraken --workspace /caminho/do/projeto --write
```

### Atalhos da TUI

- `Ctrl+Enter` — envia a mensagem ou executa o objetivo
- `Enter` — insere uma nova linha
- `/plan` — muda para o modo plan
- `/plan <objetivo>` — muda de modo e executa o objetivo
- `/chat` — volta para o modo chat
- `/chat <mensagem>` — muda de modo e envia a mensagem
- `Esc` — cancela a execução atual
- `↑`/`↓`, `Ctrl+U`/`Ctrl+D` — rola resultados longos
- `c` — continua a sessão após concluir
- `r` ou `Enter` — inicia uma sessão nova após concluir
- `q` — sair (após concluir)
- `Ctrl+C` — sair a qualquer momento

## Build

```bash
go build -o bin/kraken ./cmd/kraken
./bin/kraken
```

## Desenvolvimento

```bash
gofmt -w ./cmd ./internal
go test -race ./...
go vet ./...
```

## Evolução planejada

A especificação canônica da próxima evolução do modo plan está em
[`docs/kraken-2.0/specification.md`](docs/kraken-2.0/specification.md). O
[`backlog`](docs/kraken-2.0/backlog.md) divide a implementação em mudanças
incrementais; ele não descreve funcionalidades já disponíveis no `main`.
