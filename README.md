# Kraken

Orquestrador de desenvolvimento pelo terminal, com TUI em Textual e núcleo Python. Planeja com o usuário, revisa os contratos com contexto novo e exige aprovação explícita antes de distribuir trabalho a agentes especializados.

A versão `0.1.0` implementa o fluxo inicial e é validada em Linux com modelos controlados, Docker e Git reais em projetos descartáveis. Compatibilidade com provedores reais, macOS e WSL2 ainda exige validação. O runner Bash antigo está preservado como legado; a nova versão não usa Claude Code, Codex CLI ou claude-code-template.

## Instalação

Requisitos: Python >=3.12, Git e um contexto Docker local com socket Unix. A imagem do projeto precisa estar preparada localmente e identificada por digest. Não há pull, build ou execução direta no host como fallback durante uma tarefa.

```sh
python3.12 -m venv .venv
.venv/bin/python -m pip install -c requirements.lock .
.venv/bin/kraken --help
```

Dependências diretas: **Textual** para a interface e **HTTPX** para streaming HTTP assíncrono. Persistência, configuração, concorrência e testes do núcleo usam a biblioteca padrão. `requirements.lock` fixa as 15 distribuições diretas/transitivas; ferramentas de build ficam no ambiente de build.

## Uso

Os comandos abaixo são instruções para uso futuro. Eles não foram executados contra tarefas reais deste repositório.

```sh
kraken init /caminho/do/projeto
kraken tui /caminho/do/projeto
kraken status /caminho/do/projeto
```

`init` cria configuração local sem sobrescrever uma existente e exclui `.kraken/runtime/` pelo arquivo local de exclusões do Git. Não cria commit, backlog fictício ou branch. Execução exige ao menos um commit e ausência de alterações de código pendentes; a inspeção de planejamento é de leitura.

Na TUI:

1. Informe o objetivo e responda às decisões do planejamento. **Enviar à API** pode gerar custo; abrir a TUI não chama modelos automaticamente.
2. Confira o plano integral, incluindo exclusões, tarefas, dependências, imagem, comandos, time, modelos e limites.
3. Solicite **Revisar (API)**. QA e revisor avaliam a especificação em contextos novos. Achados impeditivos bloqueiam aprovação.
4. Use **Aprovar plano** e confirme a revisão apresentada. Alterar plano, configuração ou skills invalida a autorização anterior.
5. **Executar/retomar** inicia as tarefas elegíveis. Acompanhe tarefas, agentes, comandos, evidências e consumo. Pausar/cancelar são controles separados.

Os resultados aparecem em uma branch local `kraken/<sessão>/<revisão>`. O checkout e a branch original do usuário são preservados. Não há push, merge na principal, deploy ou publicação automática.

Também é possível importar um plano estruturado, sem aprová-lo:

```sh
kraken import-plan /caminho/do/projeto /caminho/do/plan.json
kraken health /caminho/do/projeto
```

`health` exige plano aprovado e checks de saúde declarados. Não chama modelos. Sua execução não aprova tarefas nem cria monitoramento contínuo.

## Configuração de APIs

```sh
export OPENAI_BASE_URL=https://endpoint-compativel.example/v1
export OPENAI_MODEL=modelo-escolhido
# Forneça OPENAI_API_KEY pelo mecanismo seguro do seu ambiente.
```

O Kraken lê essas variáveis explicitamente. A interface inicial é **Chat Completions**, com streaming SSE e chamadas de ferramentas; não é a API Responses. O provedor é verificado antes da implementação; modelos sem as capacidades necessárias são rejeitados, sem substituição silenciosa.

`.kraken/project.toml` pode definir defaults e overrides por papel:

```toml
[provider]
base_url = "https://endpoint-compativel.example/v1"
model = "modelo-padrao"
api_key_env = "OPENAI_API_KEY"
timeout = 60
# Opcionais: preços para estimativas, fornecidos por você e revisados no plano.
# input_usd_per_million = 1.0
# output_usd_per_million = 3.0

[roles.qa.provider]
model = "modelo-revisor"
api_key_env = "QA_API_KEY"

[execution]
docker_context = "default"
```

Precedência: defaults internos, tabela `[provider]`, variáveis globais `OPENAI_BASE_URL`/`OPENAI_MODEL`, depois override do papel. O arquivo contém **nomes** de variáveis de credenciais, nunca seus valores. HTTP sem TLS só é aceito em loopback para provedores locais. Para endpoint que não exige autenticação, defina explicitamente `api_key_env = ""`.

## Papéis, skills e artefatos

Papéis disponíveis: `product`, `orchestrator`, `backend`, `frontend`, `platform`, `qa`, `reviewer`, `security` e `ux`. O planejamento seleciona os executores necessários; QA e revisão permanecem separados da implementação. Perfis adicionais podem ser declarados em `[roles.nome]`, com `tools`, `skill` e tabela `provider` opcionais.

Skills próprias do projeto ficam em `.kraken/skills/<nome>/SKILL.md` e são selecionadas por `skill = "nome"`. O conteúdo e os perfis entram no snapshot aprovado. Skills fornecem procedimentos, não permissões. A detecção de incompatibilidade é básica; referências a outros runtimes precisam de adaptação e revisão. As skills padrão são procedimentos próprios do Kraken, informados pelas [referências da arquitetura](docs/architecture.md).

```text
.kraken/
  project.toml       configuração sem segredos
  plan.json          fonte estruturada do plano e das tarefas
  plan.md            visualização derivada; editar esta cópia não altera contratos
  skills/            procedimentos locais opcionais
  runtime/           SQLite, eventos, evidências e workspaces; não versionar
```

O plano inclui fixtures opcionais aprovadas por hash. Agentes não podem alterá-las. Checks têm categoria `behaviour`, `maintainability` ou `architecture` e momento `before_integration`, `after_integration` ou `health`. Veja o [contrato e os exemplos](docs/usage.md).

## Limites e recuperação

Defaults: concorrência 2, 100 chamadas por sessão, 30 minutos de tempo acumulado em operações, 120 segundos por comando, 10 minutos por tentativa, 3 tentativas por tarefa, 20 turnos por atribuição e 4096 tokens solicitados de saída por chamada. Planejamento e revisão também consomem a sessão. Operações paralelas acumulam tempo de forma conservadora; isso não é um cronômetro exato de tempo de parede.

Sem preços/uso suficientes, o custo é indicado como indisponível. A aprovação exige `counter_only = true`: consentimento explícito para limites verificáveis sem teto financeiro garantido. Com preços configurados, o sistema pode estimar custo e aplicar `max_estimated_cost_usd` ao despacho com reservas concorrentes. A reserva usa uma aproximação conservadora do input, não um tokenizer universal; cobrança real, chamadas em voo e uso ausente podem divergir. Não há garantia de teto financeiro rígido.

Pausar impede novos despachos e aguarda o trabalho em voo. Cancelar solicita interrupção e limpeza de containers próprios; chamadas já enviadas podem gerar custo. Retomar preserva tarefas concluídas da mesma revisão. Tentativas interrompidas ficam bloqueadas até reconciliação explícita. A TUI permite nova tentativa quando os efeitos estão restritos ao workspace isolado e não há containers pendentes; transferência Git incerta exige inspeção manual, sem repetição cega.

Uma nova revisão do plano cria outro namespace de trabalho/branch e exige revisão e aprovação integrais. Resultados anteriores ficam preservados; o MVP não reaproveita automaticamente tarefas entre revisões. Para alterar uma base já entregue, o usuário deve escolher conscientemente o commit/branch de partida.

## Desenvolvimento e testes

```sh
python3.12 -m venv .venv
.venv/bin/python -m pip install -c requirements.lock -e .
.venv/bin/python -m unittest discover -s tests -v
```

Testes de integração Docker são opt-in e usam exclusivamente projetos temporários:

```sh
docker --context default pull busybox@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec
KRAKEN_TEST_DOCKER=1 .venv/bin/python -m unittest discover -s tests -v
```

Sem essa variável, os testes Docker são **ignorados**, não aprovados. Nenhum teste chama modelo remoto. [Experimentos iniciais](experiments/README.md) são separados dos testes do produto. Veja o [relatório de validação](docs/validation.md) e o [mapeamento do artigo de Harness Engineering](docs/harness.md).

## Limitações da primeira versão

- Execução de comandos sem rede; dependências precisam estar na imagem preparada. Containers não garantem isolamento absoluto.
- Inspeção e contexto são limitados em tamanho; excesso é diagnosticado, não enviado sem limite ao provedor.
- Não há importação automática do backlog/template antigo. O backlog anterior pode ser referência para um novo plano revisado.
- Compatibilidade financeira, semântica de skills e qualidade dos critérios exigem revisão; prompts e sensores não substituem intenção humana clara.
- Provedor real, macOS/WSL2 e aceite completo do MVP continuam pendentes. Não usar as fixtures como prova de desempenho autônomo com qualquer modelo.
