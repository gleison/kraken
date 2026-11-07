# Backlog incremental do Kraken 2.0

Status: aprovado em 2026-11-06.

Cada item deve ser implementado e validado separadamente. O próximo item só
começa com o `main` verde e o critério de aceite do anterior satisfeito. A branch
`kraken/tui-api-orchestrator` não deve ser consultada nem reutilizada.

## Marco 1 — Contratos fundamentais

### K-001 — Registrar a especificação canônica

Criar especificação, glossário, ADRs, backlog e referências normativas.

- Dependências: nenhuma.
- Aceite: decisões da entrevista estão explícitas e os documentos são
  navegáveis a partir do README.
- Verificação: links locais válidos, revisão de consistência e `git diff --check`.

### K-002 — Separar contratos de implementações

Reorganizar interfaces e concretos atuais sem mudança de comportamento.

- Dependências: K-001.
- Aceite: planner, executor, modelos, ferramentas, armazenamento e runner
  dependem de portas em arquivos separados; chat e plan atuais são preservados.
- Testes: suíte atual, contratos e `go test -race ./...`.

### K-003 — Criar o domínio do plano executável

Introduzir Activity, dependências, papel, escopo, feedforward, critérios,
sensores, capacidades, artefatos e orçamento.

- Dependências: K-002.
- Aceite: tipos validáveis e snapshots sem aliases mutáveis.
- Testes: construção válida, campos inválidos, clone e serialização.

### K-004 — Validar o DAG

Implementar ordenação e validação estrutural do plano.

- Dependências: K-003.
- Aceite: detectar ciclos, IDs duplicados, dependências ausentes, atividades
  inalcançáveis e conflitos declarados.
- Testes: tabelas com DAGs válidos e cada classe de erro.

### K-005 — Modelar a máquina de estados

Definir estados e transições de planejamento, aprovação e execução.

- Dependências: K-003.
- Aceite: transições inválidas falham; eventos produzem snapshots imutáveis.
- Testes: matriz de transições e corrida sobre publicação de snapshots.

### K-006 — Implementar configuração em camadas

Combinar defaults, usuário, projeto e plano aprovado.

- Dependências: K-002.
- Aceite: precedência determinística, schema estrito, hash efetivo e proibição
  de segredos.
- Testes: merge, validação, valores desconhecidos e estabilidade do hash.

## Marco 2 — Persistência e dependências normativas

### K-007 — Criar a porta de persistência

Definir contratos para sessões, eventos, planos, evidências e artefatos.

- Dependências: K-003, K-005.
- Aceite: domínio e casos de uso não importam SQLite.
- Testes: suíte de contrato com implementação em memória.

### K-008 — Implementar SQLite e diretórios do OS

Adicionar repositório SQLite, migrações e artifact store.

- Dependências: K-007.
- Aceite: caminhos nativos, `KRAKEN_DATA_DIR`, transações e artefatos externos.
- Testes: migração, rollback, corrupção tratada e diretório temporário.

### K-009 — Implementar auditoria estruturada

Persistir decisões, ferramentas, sensores e transições.

- Dependências: K-008.
- Aceite: redação de segredos, artefatos referenciados e retenção configurável.
- Testes: redaction, ordenação, concorrência e retenção.

### K-010 — Vendorizar referências oficiais

Incorporar grill-me, grill-with-docs, dependências e EGM com manifesto.

- Dependências: K-006.
- Aceite: origem, commit, licença e hash de cada recurso registrados.
- Testes: integridade, dependências ausentes e alteração indevida.

### K-011 — Criar catálogo de skills

Carregar frontmatter, dependências, recursos e schemas via `go:embed`.

- Dependências: K-010.
- Aceite: resolução determinística e detecção de ciclos.
- Testes: catálogo válido, alias, ciclo e recurso ausente.

## Marco 3 — Perfis e planejamento

### K-012 — Criar perfis de papéis

Modelar papéis base e condicionais como personas stateless.

- Dependências: K-006, K-011.
- Aceite: missão, limites, skills, ferramentas, rubrica e saída por perfil.
- Testes: seleção por escopo e capacidade proibida.

### K-013 — Implementar roteamento de modelos

Selecionar provedor, modelo e parâmetros por papel.

- Dependências: K-002, K-012.
- Aceite: uma configuração única continua válida; credenciais ficam no broker.
- Testes: fallback, override e provedor indisponível.

### K-014 — Implementar protocolo de grilling

Criar árvore de decisões, frontier e rodadas persistíveis.

- Dependências: K-005, K-008, K-011.
- Aceite: grill-with-docs com workspace; grill-me sem projeto; perguntas
  independentes agrupadas e dependentes bloqueadas.
- Testes: árvores ramificadas, recomputação da frontier e retomada.

### K-015 — Criar ferramentas somente leitura do planner

Expor workspace e pesquisa externa por portas auditáveis.

- Dependências: K-002, K-014.
- Aceite: sem escrita, shell, credenciais ou rede genérica; fontes registradas.
- Testes: confinamento de caminhos, capacidades negadas e citações.

### K-016 — Persistir rascunhos de documentação

Manter especificação, glossário e ADRs fora do workspace antes da aprovação.

- Dependências: K-008, K-014.
- Aceite: rejeitar plano não modifica Git; reinício preserva rascunhos.
- Testes: ciclo criar/retomar/rejeitar/aprovar.

### K-017 — Implementar Goldfish isolados

Executar revisores com persona, alvo, rubrica e reading list explícitos.

- Dependências: K-011, K-013, K-015.
- Aceite: contexto fresco e findings estruturados.
- Testes: ausência de histórico herdado, fan-out e falha técnica.

### K-018 — Implementar síntese Elephant

Agregar findings e controlar convergência EGM.

- Dependências: K-017.
- Aceite: deduplicação, duas rodadas limpas, limite de cinco e um retry técnico.
- Testes: convergência, trade-off, `not run` e limite excedido.

### K-019 — Implementar aprovação versionada

Vincular autorização humana à revisão e aos hashes exibidos.

- Dependências: K-004, K-006, K-016, K-018.
- Aceite: nenhuma execução sem aprovação; mudança material invalida autorização.
- Testes: hash estável, mutações materiais e não materiais.

### K-020 — Implementar gestão de sessões

Listar, inspecionar, retomar e descartar sessões.

- Dependências: K-008, K-019.
- Aceite: associação ao workspace e confirmação para ações destrutivas.
- Testes: múltiplos projetos, sessão incompatível e descarte.

## Marco 4 — Agentes e Harness

### K-021 — Definir handoff e feedback estruturados

Criar schemas de feedforward, resultado, evidência, risco e incerteza.

- Dependências: K-003, K-012.
- Aceite: campos obrigatórios validados e texto livre apenas complementar.
- Testes: payloads completos, parciais e incompatíveis.

### K-022 — Criar contratos do Harness

Separar guides, sensors, evidências e gates.

- Dependências: K-006, K-021.
- Aceite: comportamento, arquitetura e manutenibilidade são independentes.
- Testes: composição de perfil e seleção de sensores.

### K-023 — Criar o contrato SandboxRunner

Definir lifecycle, mounts, rede, ambiente, comandos e artefatos.

- Dependências: K-002, K-006.
- Aceite: orquestrador desconhece Lima; fake cobre todo o contrato.
- Testes: suíte de contrato do runner.

### K-024 — Implementar sensores determinísticos

Executar checks configurados por uma porta de runner.

- Dependências: K-022, K-023.
- Aceite: checks rápidos primeiro e evidência com comando, saída e artefatos.
- Testes: sucesso, falha, timeout, cancelamento e saída limitada.

### K-025 — Executar o DAG serialmente

Orquestrar atividades aprovadas em ordem topológica.

- Dependências: K-004, K-005, K-019, K-021, K-023, K-024.
- Aceite: dependências e falhas respeitadas; chat independente.
- Testes: runner fake, falha intermediária e cancelamento.

### K-026 — Implementar autocorreção

Converter falhas em feedback e tentativas com contexto fresco.

- Dependências: K-024, K-025.
- Aceite: máximo padrão de três; nova decisão reabre planejamento.
- Testes: correção bem-sucedida, limite e escalação imediata.

### K-027 — Implementar orçamentos

Controlar chamadas, tokens, tempo, concorrência, armazenamento e tentativas.

- Dependências: K-009, K-013, K-025.
- Aceite: limite pausa com estado recuperável e nunca integra incompleto.
- Testes: cada dimensão, estimativa e retomada.

## Marco 5 — Git e sandbox Lima

### K-028 — Implementar worktrees isolados

Criar branch e worktree por atividade.

- Dependências: K-004, K-007.
- Aceite: branch original intocada e sobreposição detectada.
- Testes: criação, limpeza, dirty tree, conflito e interrupção.

### K-029 — Implementar artefatos verificados

Baixar componentes de origens permitidas e validar integridade.

- Dependências: K-006, K-009.
- Aceite: versão, checksum/assinatura, manifesto e rollback.
- Testes: download válido, truncado, adulterado e origem proibida.

### K-030 — Implementar setup e doctor

Preparar Lima, guest tools e imagem após consentimento.

- Dependências: K-029.
- Aceite: `/plan` não baixa silenciosamente; diagnóstico acionável.
- Testes: instalação limpa, já instalada, incompatível e offline.

### K-031 — Implementar lifecycle da VM

Criar, iniciar, verificar, reutilizar e remover VM Linux.

- Dependências: K-023, K-030.
- Aceite: suporte a macOS e Linux e falhas de VZ/KVM explicadas.
- Testes: integração real por sistema operacional.

### K-032 — Executar worktrees dentro da VM

Montar apenas recursos autorizados e executar comandos nativos.

- Dependências: K-028, K-031.
- Aceite: `go test` funciona; host e credenciais não são visíveis.
- Testes: leitura/escrita permitida, escape negado e subprocessos.

### K-033 — Restringir rede e ambiente

Separar aquisição de dependências da execução offline.

- Dependências: K-032.
- Aceite: rede bloqueada nos checks, ambiente mínimo e segredos rejeitados.
- Testes: DNS/TCP bloqueados, variáveis ausentes e fase permitida auditada.

### K-034 — Provisionar toolchains por hash

Criar ambientes derivados imutáveis por projeto.

- Dependências: K-029, K-031, K-033.
- Aceite: versões fixadas, cache reutilizável e nenhuma atualização silenciosa.
- Testes: provisionamento, cache hit, hash diferente e rollback.

## Marco 6 — Integração e qualidade final

### K-035 — Criar branch integrada da sessão

Combinar commits na ordem do DAG e repetir sensores.

- Dependências: K-024, K-026, K-028, K-032.
- Aceite: conflitos preservam resultados e não alteram a branch original.
- Testes: integração limpa, conflito e reexecução de gates.

### K-036 — Implementar QA e revisão finais

Avaliar o resultado completo com agentes independentes.

- Dependências: K-017, K-022, K-035.
- Aceite: QA cobre comportamento; Revisor cobre arquitetura/manutenibilidade.
- Testes: findings, aprovação, correção e contextos isolados.

### K-037 — Integrar na branch original

Aplicar resultado aprovado somente sobre a base autorizada.

- Dependências: K-036.
- Aceite: operação atômica, branch limpa e commit-base idêntico.
- Testes: fast-forward, branch divergente, dirty tree e rollback.

### K-038 — Implementar cancelamento e recuperação

Preservar estado, commits e evidências em interrupções.

- Dependências: K-020, K-025, K-028, K-031.
- Aceite: `Esc` cancela cooperativamente; reinício não repete concluído.
- Testes: cancelamento em cada fase, crash simulado e retomada.

## Marco 7 — TUI e automação

### K-039 — Adicionar entrevista e aprovação à TUI

Exibir frontier, documentos, findings e revisão do plano.

- Dependências: K-014, K-018, K-019.
- Aceite: visual atual e chat preservados; rodada só avança completa.
- Testes: update/view para perguntas, revisão e aprovação.

### K-040 — Adicionar acompanhamento da execução

Mostrar atividade, papel, sensores, tentativas, orçamento e eventos.

- Dependências: K-009, K-025, K-027.
- Aceite: detalhes sob demanda e apenas snapshots imutáveis.
- Testes: resize, scroll, concorrência e eventos extensos.

### K-041 — Adicionar gestão de sessões à TUI

Listar, inspecionar, retomar e descartar no modo plan.

- Dependências: K-020, K-038.
- Aceite: Kraken inicia em chat e apenas sugere retomada em `/plan`.
- Testes: sessão pendente, incompatível e descarte confirmado.

### K-042 — Implementar paralelismo seguro

Executar atividades independentes sem conflito.

- Dependências: K-025, K-028, K-035, K-038.
- Aceite: concorrência configurável e integração determinística serializada.
- Testes: fan-out/fan-in, limites, sobreposição e cancelamento.

### K-043 — Testar Lima no CI de Linux e macOS

Adicionar jobs de integração reais.

- Dependências: K-031 a K-034, K-038.
- Aceite: bootstrap, isolamento, `go test`, rede, cancelamento e recuperação.
- Testes: workflows passam nos dois hosts suportados.

### K-044 — Automatizar atualizações upstream

Verificar semanalmente Skills, EGM, Lima e imagens.

- Dependências: K-010, K-029, K-043.
- Aceite: PR automática com diff, manifesto e testes; merge sempre manual.
- Testes: upstream igual, atualização válida e atualização incompatível.
