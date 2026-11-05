# Fechamento das lacunas da implementação inicial

Esta auditoria substitui a interpretação anterior de que a existência de código em K-01–K-16 bastava para concluir o escopo. Cada frente abaixo exige comportamento verificável, sem confundir implementação e validação externa.

| Frente | Comportamento implementado | Evidência específica |
| --- | --- | --- |
| Planejamento e revisão | Rodadas limitadas de revisão independente, correção técnica e nova revisão; decisão humana quando falta intenção; sem aprovação automática. Campos de escopo, decisões, imagem, fixtures e limites não podem ser alterados pela correção automática. | `test_review_repairs_technical_findings_and_rechecks_with_fresh_context`, `test_review_requests_only_missing_human_decision`, `test_automatic_repair_cannot_change_scope_or_budget`, `test_review_round_limit_keeps_plan_unapproved`. |
| Orçamento do planejamento | O rascunho do modelo não pode ampliar os limites das chamadas usadas para planejá-lo/revisá-lo. Orçamento definido pelo usuário em configuração, separado dos limites de execução a aprovar. | `test_unapproved_draft_cannot_raise_planning_api_budget`. |
| Replanejamento | Seleção de resultados potencialmente reutilizáveis por contrato e dependências; invalidação transitiva; nenhum resultado é considerado concluído na nova revisão antes de nova aprovação, checks e QA/revisão independentes. | `test_new_revision_revalidates_and_reuses_unchanged_completed_task`, `test_replan_invalidates_changed_task_and_transitive_dependents_only`. |
| Recuperação Git | Journal durável com commit, base, ref anterior, contexto e IDs das evidências; retomada idempotente das fases; compare-and-swap de refs, sem sobrescrever divergências. | Quatro testes `test_resume_after_*`, `test_divergent_destination_is_never_overwritten`, `test_invalidated_evidence_prevents_recovery`. |
| Harness | Resultados normalizados para comando funcional, unittest, pytest e JUnit; contagens separadas de pass/fail/error/skip/xfail/xpass/disabled; artefato exclusivo da execução e rejeição de relatório ausente/inconsistente. | `tests/test_harness.py`, `test_junit_is_collected_from_this_container_before_removal`. |
| Skills | Origem, versão, licença declarada, runtime, ferramentas, executáveis e variáveis necessários. Requisitos desconhecidos/incompatíveis não são ignorados; nenhum valor secreto é persistido. | `tests/test_skills.py`; preflight de executáveis na imagem aprovada. |
| TUI e sessões | Catálogo local de projetos, seleção, criação e retomada de sessões. Planos, aprovações, consumo, evidências e workspaces permanecem separados. A troca de sessão não inicia tarefas nem API. | `tests/test_projects.py`, `test_project_manager_lists_sessions_and_returns_selection_without_running`, `test_main_tui_can_request_new_session_without_implicit_api`. |

## O que não se pode concluir desses testes

As respostas de modelos são controladas. Isso testa regras e transições, não demonstra que um LLM real fará bom planejamento ou boa revisão. Não há promessa de eliminar toda intervenção humana ou garantir um teto de faturamento de uma API externa.

A matriz de execução real permanece explícita:

- Linux: suíte completa com Docker/Git locais, incluindo testes de recuperação.
- macOS: pipeline de núcleo/TUI em runner hospedado e integração Docker em runner macOS apropriado, por acionamento explícito.
- WSL2: job de integração em runner Linux/WSL2 que verifica o kernel real, por acionamento explícito.
- Provedor real: OpenAI é a preferência indicada pelo usuário; modelo, credencial e orçamento ainda não foram definidos. Nenhuma chamada paga está autorizada por uma suposição.

Código de CI configurado não é resultado de CI. K-17 e o aceite final de K-18 só fecham com evidências das execuções correspondentes.
