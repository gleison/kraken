# Kraken 2.0 — Especificação canônica

Status: aprovado para implementação incremental em 2026-11-06.

Este documento registra o produto acordado para a evolução do modo plan do
Kraken. Ele é normativo para as atividades listadas em
[`backlog.md`](backlog.md). Funcionalidades descritas aqui só passam a existir
quando a respectiva atividade for concluída no `main`.

Documentos relacionados:

- [`glossary.md`](glossary.md): vocabulário do domínio;
- [`backlog.md`](backlog.md): ordem incremental de implementação;
- [ADR 0001](../adr/0001-preserve-go-tui-and-chat-default.md): Go, TUI e chat;
- [ADR 0002](../adr/0002-use-grilling-and-elephant-goldfish-for-planning.md):
  grilling e EGM;
- [ADR 0003](../adr/0003-use-executable-dag-and-harness-gates.md): DAG e Harness;
- [ADR 0004](../adr/0004-use-lima-linux-vm-as-sandbox.md): sandbox Lima;
- [ADR 0005](../adr/0005-persist-sessions-outside-the-workspace.md): persistência;
- [ADR 0006](../adr/0006-isolate-activities-with-git-worktrees.md): worktrees;
- [ADR 0007](../adr/0007-separate-ports-and-publish-immutable-snapshots.md):
  portas e snapshots;
- [ADR 0008](../adr/0008-vendor-and-review-upstream-resources.md): recursos upstream.

## 1. Objetivo e escopo

O Kraken continua sendo um aplicativo Go com TUI Bubble Tea. O modo chat
permanece a experiência padrão e direta. O novo pipeline é ativado apenas por
`/plan` ou `kraken --plan` e transforma uma especificação em um plano aprovado,
executável, verificável e recuperável.

O pipeline completo possui duas macroetapas:

1. planejamento, que fecha lacunas e obtém aprovação humana;
2. execução, que implementa o plano aprovado com agentes especializados,
   isolamento, sensores e ciclos limitados de correção.

Não fazem parte da primeira versão:

- Windows;
- workloads que exijam APIs ou toolchains exclusivos do macOS;
- injeção de segredos na sandbox;
- fallback silencioso para execução sem isolamento forte;
- redesenho amplo da TUI;
- qualquer código, requisito ou teste da branch
  `kraken/tui-api-orchestrator`, que deve ser ignorada.

## 2. Referências normativas

| Referência | Origem | Baseline observado em 2026-11-06 |
|---|---|---|
| Harness Engineering | <https://martinfowler.com/articles/harness-engineering.html> | artigo publicado em 2026-04-02 |
| Matt Pocock Skills | <https://github.com/mattpocock/skills> | `4ae2317ddb22b046e26455e4f9c9b3aaaf62212d` |
| Elephant–Goldfish Model | <https://github.com/google/elephant-goldfish-model> | `6a8001ea31ccf72430ba0603a467ec6df9f69dd7` |
| Lima | <https://github.com/lima-vm/lima> | `800a96b14e96b5d2ceb20ab2602147c1e79224ea` |

Os hashes acima documentam o estado consultado durante a especificação. A
atividade de vendorização deve validar conteúdo, licença e dependências e então
registrar o pin efetivamente incorporado. Sessões aprovadas sempre referenciam
hashes exatos, nunca branches móveis.

## 3. Experiência e modos

### 3.1 Chat

- É o modo inicial.
- Mantém conversa direta, histórico e ferramentas atuais.
- Mantém o campo de escrita visível durante geração e preserva o rascunho.
- Não executa grilling, EGM, DAG ou pipeline de agentes.

### 3.2 Plan

- É ativado explicitamente por `/plan`, `/plan <objetivo>` ou `--plan`.
- Conduz planejamento, revisão, aprovação, execução e acompanhamento.
- Preserva a linguagem visual atual da TUI.
- Expõe detalhes de perguntas, plano, atividades, sensores, tentativas,
  orçamento e evidências sob demanda.
- Recebe do orquestrador apenas snapshots imutáveis.

## 4. Planejamento

### 4.1 Seleção da skill

- Com workspace/documentação persistente, usar `grill-with-docs`.
- Sem projeto associado, usar `grill-me`.
- Incorporar as skills oficiais completas, inclusive aliases, dependências e
  recursos transitivos.
- Skills são procedimentos declarativos. O orquestrador Go controla estado,
  rodadas, permissões e limites; Markdown nunca é executado como código.

### 4.2 Entrevista

- Modelar decisões como uma árvore.
- Em cada rodada, apresentar todas as perguntas independentes da frontier.
- Perguntas dependentes ficam bloqueadas até suas premissas serem resolvidas.
- Fatos são investigados por agentes auxiliares; decisões pertencem ao usuário.
- Investigação tem somente leitura do workspace e pesquisa web auditada.
- Não há shell, escrita, credenciais ou rede genérica durante o planejamento.
- Fontes externas utilizadas devem ser registradas.

### 4.3 Documentação viva

Glossário, ADRs e especificação produzidos por `grill-with-docs` ficam como
rascunhos no armazenamento da sessão. Eles só entram no Git como a primeira
atividade de um plano aprovado. Rejeitar um plano não modifica o workspace.

### 4.4 Elephant–Goldfish

O modelo deve ser aplicado de acordo com a referência oficial:

- Elephant mantém contexto, elabora a especificação e resolve findings.
- Goldfish são revisores adversariais isolados, não implementadores.
- Cada Goldfish recebe persona, alvo, rubrica e reading list explícitos.
- Revisores não herdam a conversa do Elephant.
- Revisões podem ocorrer em paralelo e são sintetizadas e deduplicadas.
- Uma falha técnica de reviewer recebe uma tentativa adicional; persistindo,
  registra-se `not run`.
- Convergência exige duas rodadas limpas consecutivas.
- O limite é de cinco rodadas; trade-offs e falta de convergência escalam ao
  usuário.

### 4.5 Aprovação

Antes de qualquer escrita ou comando mutável, a TUI apresenta:

- especificação e documentos propostos;
- plano e DAG;
- capacidades e acessos;
- perfis e modelos;
- sensores e evidências esperadas;
- orçamento estimado;
- hashes de skills, configuração, workspace e runtime.

A aprovação é vinculada à revisão exata do plano. Mudança material invalida a
aprovação. Após aprovação, o pipeline trabalha autonomamente e só pausa por
ação destrutiva, decisão não prevista, conflito de requisitos, alteração do
plano ou esgotamento de limites.

## 5. Plano executável

O plano não é uma lista de prompts. É um DAG de atividades com dependências
explícitas. Cada atividade registra, no mínimo:

- identificador, título e objetivo;
- dependências;
- papel executor;
- escopo e caminhos permitidos;
- feedforward e decisões relevantes;
- critérios de aceite;
- sensores de comportamento, arquitetura e manutenibilidade;
- capacidades requeridas;
- artefatos e evidências esperados;
- orçamento e política de correção.

O scheduler começa serial. O contrato já permite paralelismo futuro somente
entre atividades sem dependência, sobreposição de arquivos ou conflito de
recursos.

## 6. Time e agentes

Papéis são personas stateless, criadas conforme a necessidade. Perfis definem
missão, limites, ferramentas, skills, rubrica e formato de saída. Skills
encapsulam procedimentos reutilizáveis; não são sinônimo de papel.

Papéis base:

- Analista de Produto: participa do planejamento.
- Desenvolvedor de Backend: entra quando houver backend.
- Desenvolvedor de Frontend: entra quando houver frontend.
- QA: gate independente de comportamento.
- Revisor de Código: gate independente de arquitetura e manutenibilidade.

Papéis condicionais:

- Platform/DevOps;
- Segurança;
- UX/Acessibilidade;
- Dados.

Cada perfil pode selecionar provedor, modelo e parâmetros. O padrão é reutilizar
o cliente OpenAI-compatible configurado. Credenciais não pertencem aos perfis.

## 7. Feedforward, feedback e Harness

Cada handoff ao executor é autocontido e contém contexto mínimo, decisões,
papel, escopo, restrições, ferramentas e definição de pronto.

Cada executor retorna dados estruturados:

- alterações e commits;
- decisões locais;
- comandos executados;
- evidências;
- critérios atendidos;
- riscos e incertezas;
- capacidades adicionais solicitadas.

Texto livre pode acompanhar, mas não substitui campos obrigatórios. O Kraken
valida evidências diretamente.

O Harness combina:

- guides/feedforward antes da ação;
- sensors/feedback depois da ação;
- sensores computacionais antes dos inferenciais;
- gates separados de comportamento, arquitetura e manutenibilidade.

Sensores rápidos rodam após cada atividade. Após integrar a branch da sessão,
todos os sensores determinísticos rodam novamente, seguidos por QA e revisão
independentes sobre o conjunto completo.

Falhas geram feedback estruturado e nova tentativa em contexto fresco. O padrão
é de três tentativas por atividade. Decisões novas pausam a execução e reabrem
uma revisão específica do planejamento. Alterações recorrentes no Harness são
sugeridas de forma auditável; agentes nunca mudam silenciosamente o próprio
Harness.

## 8. Git e integração

- Cada atividade de implementação usa branch e worktree próprios.
- Commits por atividade formam a unidade rastreável de resultado.
- Uma branch de sessão integra os commits na ordem do DAG.
- A branch original só é alterada após todos os gates passarem.
- Integração automática exige branch original limpa e no commit-base aprovado.
- Divergência ou conflito preserva a branch de resultado e pausa a sessão.
- Nenhuma operação sobrescreve mudanças concorrentes do usuário.

## 9. Sandbox e runtime

### 9.1 Fronteira

Lima é o backend padrão de sandbox em macOS e Linux. Ele gerencia uma VM Linux
que executa toolchains, comandos e código do projeto. Não há Docker nem runner
local menos seguro na primeira versão.

Workloads exclusivos de macOS são recusados com diagnóstico. Um backend nativo
de macOS fica para o futuro.

### 9.2 Bootstrap

`kraken setup` instala e verifica Lima, imagem e guest tools após consentimento.
`/plan` apenas detecta ausência e oferece o setup. Downloads usam origens
permitidas, versões fixadas e checksum ou assinatura. Divergência aborta antes
da execução. Versões anteriores permanecem disponíveis para rollback.

### 9.3 Ambiente da VM

- Montar somente worktree, caches e temporários autorizados.
- Não herdar ambiente, SSH, Git, tokens ou diretórios do host.
- Desabilitar rede durante implementação e testes.
- Adquirir dependências em fase explícita, auditada e cacheável.
- Rejeitar tarefas que exijam segredos na primeira versão.
- Usar imagem mínima e toolchains derivados, imutáveis e identificados por hash.

Chamadas a LLMs e pesquisa ocorrem no processo host. A VM nunca recebe tokens
de API. O host atua como broker e envia apenas operações concretas ao runner.

## 10. Persistência, recuperação e auditoria

Persistir especificação, revisão aprovada, DAG, estado, handoffs, evidências,
tentativas, orçamento e eventos.

A implementação inicial usa SQLite atrás de uma interface. Local padrão:

- macOS: `~/Library/Application Support/kraken/`;
- Linux: `$XDG_STATE_HOME/kraken` ou `~/.local/state/kraken`;
- override: `KRAKEN_DATA_DIR`.

Metadados ficam no banco; artefatos grandes ficam em arquivos referenciados. O
Kraken permite listar, inspecionar, retomar e descartar sessões. O modo inicial
continua sendo chat e a retomada é apenas sugerida ao entrar em `/plan`.

`Esc` cancela cooperativamente a operação atual e preserva estado recuperável.
Descarte definitivo é separado e confirmado. Retomada valida hashes de
workspace, configuração, skills e runtime.

Toda decisão, chamada de agente, ferramenta, sensor, transição e integração
gera evento estruturado. Segredos conhecidos são redigidos. Logs detalhados são
opt-in e têm retenção configurável.

## 11. Configuração e orçamento

A configuração efetiva combina, nesta ordem:

1. padrões incorporados;
2. preferências do usuário no diretório de dados;
3. arquivo opcional versionado no projeto;
4. revisão aprovada do plano.

Configuração é validada estritamente e não contém segredos. A sessão registra
seu hash efetivo.

Cada sessão limita chamadas, tokens, duração, concorrência, armazenamento e
tentativas. A estimativa é exibida antes da aprovação. Atingir um limite pausa
com estado recuperável; nunca reduz silenciosamente critérios de qualidade nem
integra trabalho incompleto.

## 12. Atualizações upstream

Skills do Matt Pocock, pacote EGM, Lima e imagens são dependências versionadas.
Um workflow semanal verifica upstream e abre PR com conteúdo, manifesto, diff e
testes. O CI normal falha por inconsistência local, não apenas por existir uma
versão nova. Atualizações nunca fazem merge automático nem alteram sessões já
aprovadas.

## 13. Qualidade do próprio Kraken

Cada incremento exige:

- testes unitários Go;
- testes de contrato das interfaces;
- testes de integração proporcionais ao risco;
- integração real com Lima em CI para Linux e macOS quando o runner for afetado;
- preservação do comportamento do chat e do visual geral da TUI.

O `main` deve permanecer compilável e testável entre atividades.
