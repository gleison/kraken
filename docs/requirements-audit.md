# Rechecagem dos requisitos aprovados

Auditoria de 5 de novembro de 2026, confrontando a [especificação](spec.md), os contratos do código, os testes e o [harness](harness.md) baseado no PDF fornecido. Esta matriz distingue implementação exercitada com modelos controlados de validação externa. Não constitui aceite integral do MVP.

## Correções encontradas nesta revisão

- O scheduler respeitava dependências e concorrência, mas não serializava caminhos de edição sobrepostos. Agora compara arquivos/diretórios, inclusive ancestrais, antes de despachar. Uma tarefa bloqueada por sobreposição não impede outra independente de usar a capacidade disponível.
- Falhas computacionais e reprovações de QA/revisão agora produzem contratos de correção com critérios, observação, versão, ID de evidência e próxima ação. O feedback persistido acompanha a tentativa seguinte.
- A TUI agora mostra dependências e permite selecionar evidências persistidas por tarefa, inclusive depois da retenção dos eventos. A lista lê apenas metadados; o conteúdo é carregado sob seleção. A visualização tem limite explícito de 60 mil caracteres, preservando o registro integral no SQLite.
- O planejador agora recebe modelos, ferramentas, hashes e requisitos das skills dos papéis disponíveis. O plano renderizado explicita o time derivado das tarefas, acrescido dos participantes obrigatórios de planejamento, coordenação e avaliação. Não foi alterado o esquema nem o hash de planos existentes.
- Handoffs e avaliações recebem objetivo, escopo e versão; o implementador recebe também o perfil efetivamente autorizado. O feedback inclui trechos limitados da saída, mantendo o conteúdo integral na evidência persistida. A amostragem inicial lê arquivos de forma limitada desde a abertura.
- O executor recusa execução do Kraken como root antes de acessar o Docker; assim, a identidade usada no container nunca herda UID zero do host.

## R01–R18

“Controlado” significa código implementado e comportamento exercitado em fixtures; não significa comprovação de qualidade de um LLM real.

| ID | Implementação e evidência conferidas | Estado / limite |
| --- | --- | --- |
| R01 | `ui.py`: painéis, agentes, comandos/eventos, dependências e inspeção de evidências. `test_startup_does_not_call_api_or_approve`, `test_dependencies_and_persisted_evidence_remain_inspectable_without_events`. | Controlado, incluindo redimensionamento. |
| R02 | `session.py`, skill `planning`: inspeção somente leitura, entrevista, memória de decisões, correção de plano. `test_interview_plan_fresh_reviews_and_explicit_approval`, testes de `ProjectReader`. | Controlado; eficácia da entrevista com LLM real ainda não demonstrada. |
| R03 | `store.py`, `Session.approve`, modal de confirmação; hashes do plano/configuração e revisões independentes obrigatórias. Testes de aprovação explícita, revisão obsoleta e plano alterado em disco. | Controlado; nenhuma aprovação por silêncio ou saída de modelo. |
| R04 | `Session.review`: QA e revisor com conversas novas; correções limitadas e escalonamento de intenção ausente. `test_review_repairs_technical_findings_and_rechecks_with_fresh_context`. | Controlado; teste verifica ausência do histórico da entrevista. |
| R05 | SQLite, memória de planejamento, handoff e feedback por atribuição. Testes de contexto/versão e `test_independent_qa_failure_produces_bounded_correction`. | Controlado; contexto limitado, sem promessa de memória ilimitada. |
| R06 | `Budget`, `Limits`, timeout de comandos/tentativas, turnos e número de correções. Testes de orçamento concorrente, preço desconhecido e rascunho sem poder ampliar orçamento. | Controlado; custo é estimado e não garante teto de faturamento. |
| R07 | Workspaces/container por tarefa, claims transacionais, dependências e caminhos sobrepostos. `test_two_independent_tasks_then_dependent`, `test_overlapping_paths_serialize_without_blocking_unrelated_work`. | Controlado em Linux/Docker. Recursos externos compartilhados não são disponibilizados pelo executor offline. |
| R08 | `DockerExecutor`: rede desabilitada, usuário sem root, raiz somente leitura, mounts protegidos, limites de recursos, sem socket Docker. Testes de fronteiras reais e engine/contexto inválidos. | Demonstrado em Linux; Docker no macOS e WSL2 pendentes. Shell acessa os arquivos montados do projeto; limites de staging são verificados adicionalmente pelo Git. |
| R09 | Checks normalizados, fixtures protegidas, QA/revisor em atribuições separadas e revalidação do candidato integrado. Testes de zero testes, reprovação semântica e regressão de tarefa já integrada. | Controlado; julgamento independente por IA continua falível. |
| R10 | `provider.py`: Chat Completions/SSE, ferramentas fragmentadas/intercaladas, uso, cancelamento e probe do protocolo. `tests/test_provider.py`. | Adapter implementado; compatibilidade com OpenAI real pendente. |
| R11 | `Config`, `Profile`: variáveis de endpoint/modelo/chave e overrides por papel. `test_role_override_and_skill_change_invalidate_snapshot`. | Controlado; credenciais reais não configuradas. |
| R12 | Papéis padrão e locais configuráveis; capacidades fornecidas ao planejamento; time derivado de tarefas e apresentado no plano; perfis/skills completos no modal aprovado. Testes de planejamento, renderização e configuração. | Controlado; Produto, Orquestrador, QA e Revisor obrigatórios; especialistas conforme tarefas. |
| R13 | Catálogo `SKILL.md`, extensões locais, hashes e `skill.toml`. `tests/test_skills.py`, `test_skill_executable_requirement_checked_in_approved_image`. | Compatibilidade declarada verificada; não adapta automaticamente skills de qualquer runtime. |
| R14 | Planos JSON/Markdown versionáveis; decisões e backlog em contratos; SQLite, sessões, histórico e workspaces separados. Testes de persistência, troca de sessões, replanejamento e invalidação transitiva. | Controlado; não faz commit dos documentos do usuário implicitamente. |
| R15 | `Repository`: clones isolados, staging limitado, branch Kraken, integração serializada, journal e CAS. `tests/test_recovery.py` e testes Git de integração. | Controlado em Linux; nenhum push/merge/deploy implícito. |
| R16 | `pause`, `cancel`, recuperação e decisões pendentes; controles TUI. Testes de pausa, cancelamento, reinício e quatro fases interrompidas da transferência Git. | Controlado; chamadas interrompidas podem consumir no provedor e são sinalizadas como incertas. |
| R17 | Código POSIX/Python e CI Ubuntu/macOS; jobs específicos de integração macOS e WSL2. | Parcialmente validado: Linux completo e núcleo/TUI macOS demonstrados; falta Docker no macOS e WSL2. |
| R18 | Núcleo separado da TUI, Python 3.12+, somente HTTPX e Textual como dependências diretas. `test_core_does_not_import_tui_framework`, instalação e testes do wheel. | Controlado; dependências transitivas fixadas em `requirements.lock`. |

## AC01–AC12

| Critério | Evidências principais | Resultado da auditoria |
| --- | --- | --- |
| AC01 | Aprovação ausente, mudança de plano/configuração, restauração de sessão e gate antes de escrita/comando. | Demonstrado com fixtures. |
| AC02 | Entrevista, time/capacidades, contratos, revisão sem histórico, achado técnico e decisão humana. | Fluxo demonstrado com respostas controladas; qualidade real pendente. |
| AC03 | Streaming fragmentado, argumentos inválidos, reconstrução de ferramentas e políticas de ferramentas. | Demonstrado por transporte controlado e gates de execução. |
| AC04 | Duas tarefas simultâneas isoladas, dependente posterior, claim único e sobreposição serializada. | Demonstrado com Docker/Git reais em Linux. |
| AC05 | QA reprova, evidência é vinculada ao feedback e nova tentativa é limitada; zero testes não aprova. | Demonstrado com modelos controlados. |
| AC06 | Checkout original preservado, alteração fora do contrato recusada antes de commit, checks repetidos na integração e CAS nas refs. | Demonstrado com Git real em projetos descartáveis. |
| AC07 | Falha de engine/contexto bloqueia; teste dentro do container verifica rede, mounts e usuário. | Demonstrado em Linux; matriz completa depende de AC12. |
| AC08 | Pausa sem novos despachos, cancelamento, containers próprios e journal idempotente com provas. | Demonstrado em cenários de interrupção controlados. |
| AC09 | Quotas/reservas concorrentes, preço/uso desconhecidos, eventos incertos e sanitização de credenciais de API configuradas. | Demonstrado; sem promessa de controle exato de cobrança. |
| AC10 | TUI sem chamadas de startup, confirmação explícita, resize, catálogo e evidências independentes da retenção de eventos. | Demonstrado em testes de interação headless; uso prolongado humano ainda não avaliado. |
| AC11 | Overrides, alteração de skills/manifests invalida hash, runtime/rede/ferramentas/executáveis incompatíveis bloqueiam. | Demonstrado com fixtures e imagem aprovada. |
| AC12 | Linux completo; núcleo/TUI Ubuntu e macOS em CI. Integrações completas macOS e WSL2 ainda não executadas. | **Pendente para aceite completo.** |

## Evidência de execução e pendências

Os resultados e versões das execuções estão em [validation.md](validation.md). Nenhuma tarefa real do usuário, chamada paga ou publicação de ambiente integra esta auditoria.

Permanecem abertos K-17 e o aceite de K-18: escolher modelo/limites e configurar credencial segura para validar com OpenAI; executar a integração Docker em macOS; executar a integração em WSL2 real. R10 e R17 não devem ser anunciados como plenamente comprovados enquanto essas evidências faltarem. Testes controlados não garantem ausência de defeitos ou que intervenção humana sempre será excepcional.
