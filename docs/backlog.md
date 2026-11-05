# Backlog proposto da refatoração

Status: detalhamento aprovado pelo usuário. K-01–K-16 têm implementação inicial e testes controlados. K-17 está pendente de validação real; K-18 tem documentação pronta, com aceite final dependente de K-17. A implementação segue as dependências e os gates abaixo; a aprovação não autoriza chamadas reais a provedores, tarefas reais do usuário, push ou publicação implícitos.

Cada tarefa abaixo deverá ganhar contrato executável completo no planejamento: limites concretos, modelos, skills, recursos, critérios e evidências. Os papéis são uma proposta de responsabilidade, não processos já existentes.

## Estratégia de entrega

Começar por decisões impeditivas e contratos, construir uma fatia vertical com modelo controlado e depois ampliar recuperação, paralelismo e integração. Nenhum caminho temporário pode permitir implementação sem aprovação ou execução de comandos fora do isolamento escolhido. Testes simulados e validação com provedor real serão reportados separadamente.

O “Backend” neste backlog implementa o núcleo Python; “Frontend” implementa a TUI em Textual. QA e Revisão acompanham cada entrega, com rigor proporcional ao seu impacto.

## Tarefas

### [x] K-01 — Fechar referências e decisões impeditivas

- Progresso: P01 resolvida pelo PDF e mapeamento; P02–P06 com decisões e evidências. Ver [decisões técnicas](technical-decisions.md) e [validação](validation.md).
- Dependências: nenhuma.
- Papéis: Produto e Plataforma/Harness, com revisão independente.
- Entrega: resolver P01–P06 da especificação ou explicitar uma alteração de escopo aprovada; confrontar o harness com a fonte de Fowler; escolher containers, transporte, formatos, Python e distribuição; propor limites e política de custo desconhecido.
- Aceitação: decisões e limitações justificadas, análise de dependências mínimas, cenários de Git e ambiente descritos, sem lacuna impeditiva ocultada. Experimentos técnicos futuros precisam de autorização e não executam o runner antigo.
- Evidência: documentos, resultados dos experimentos autorizados e aprovação do detalhamento. Não começar implementação com pendências disfarçadas de defaults.

### [x] K-02 — Estrutura Python e contratos de domínio

- Dependências: K-01.
- Papéis: Backend e Plataforma/Harness.
- Entrega: separação núcleo/TUI, modelos de projeto/plano/tarefa/tentativa, configuração validada e conjunto mínimo de dependências fixadas.
- Aceitação: núcleo importável sem iniciar TUI, comandos, containers ou APIs; contratos inválidos e ciclos de dependência são rejeitados; instalação reproduzível no Python escolhido.
- Evidência: testes de contratos e instalação em ambiente limpo.

### [x] K-03 — Persistência, aprovação e controle de estado

- Dependências: K-02.
- Papéis: Backend.
- Entrega: armazenamento, revisões de plano, aprovações explícitas, transições, identificadores estáveis e eventos.
- Aceitação: plano não aprovado não despacha implementação; mudança relevante invalida autorização afetada; reinício não concede autorização; despachos concorrentes não duplicam a mesma tentativa.
- Evidência: testes de transições, transações e interrupção antes/depois de persistir efeitos.

### [x] K-04 — Adapter Chat Completions

- Dependências: K-02.
- Papéis: Backend.
- Entrega: configuração padrão e por papel, streaming, tool calls, timeouts, erros e uso informado pelo provedor.
- Aceitação: chunks parciais reconstroem argumentos completos antes da execução; dados inválidos não viram comandos; falhas não causam fallback silencioso; logs não contêm credenciais.
- Evidência: servidor ou transporte controlado com streaming fragmentado, erros, cancelamento e capacidades ausentes. Compatibilidade real ficará para K-17.

### [x] K-05 — Runtime de agentes e políticas de ferramentas

- Dependências: K-03, K-04.
- Papéis: Backend e Plataforma/Harness.
- Entrega: ciclo de chamadas, contexto por atribuição, validação de resultados, limites de turnos e registro de ferramentas.
- Aceitação: ferramenta só executa com autorização; saída de modelo não altera aprovação ou política; chamadas recursivas e correções respeitam limites; operações incertas não são repetidas cegamente.
- Evidência: testes com modelos e ferramentas controlados, inclusive tentativas de ampliar permissões.

### [x] K-06 — Perfis de papéis e catálogo de skills

- Dependências: K-02, K-05.
- Papéis: Produto e Backend.
- Entrega: papéis padrão, extensões locais, referências versionadas de skills e identificação de requisitos de ferramentas.
- Aceitação: perfis herdam defaults e restringem recursos; skill alterada ou incompatível é detectada; skill não concede permissão; origens e atribuições ficam registradas.
- Evidência: fixtures de skills compatíveis/incompatíveis e cenários de overrides por papel.

### [x] K-07 — TUI inicial sobre eventos reais

- Dependências: K-03.
- Papéis: Frontend e QA.
- Entrega: seleção de projeto/sessão, painéis de tarefas, agentes, eventos, logs e decisões; entradas explícitas de aprovação.
- Aceitação: estados vêm do núcleo, não de timers decorativos; histórico extenso e terminal pequeno não bloqueiam interação; aprovação é ação identificável do usuário, nunca texto gerado.
- Evidência: testes de interação, redimensionamento, teclado, estados sem cor e sequência de eventos controlada.

### [x] K-08 — Planejamento, entrevista e contratos de tarefas

- Dependências: K-05, K-06, K-07.
- Papéis: Produto, Backend e Frontend.
- Entrega: inspeção autorizada de leitura, entrevista orientada por decisões, glossário, plano, backlog e composição do time.
- Aceitação: critério de aceitação liga-se a verificações; tarefas têm contratos e dependências válidos; incertezas impeditivas permanecem explícitas; nenhuma implementação ocorre durante planejamento.
- Evidência: cenário completo de entrevista com decisão corrigida pelo usuário e inspeção limitada ao projeto.

### [x] K-09 — Revisão do plano e aprovação versionada pela TUI

- Dependências: K-08.
- Papéis: Produto, Backend, Frontend e Revisão.
- Entrega: revisores com contexto novo, achados, correções, apresentação consolidada e aprovação vinculada à revisão.
- Aceitação: revisor detecta lacuna inserida no contrato sem receber a conversa; achado impeditivo bloqueia execução; silêncio ou retomada não aprovam; alteração de escopo exige nova aprovação.
- Evidência: cenário de revisão, aprovação e invalidação posterior. Completa AC01 e AC02 junto a K-03.

### [x] K-10 — Containers e workspaces isolados

- Dependências: K-01, K-03, K-05.
- Papéis: Plataforma/Harness e Backend.
- Entrega: backend selecionado, preparação reproduzível, mounts, restrição de rede, processos, arquivos e limpeza controlada por tarefa.
- Aceitação: execução não alcança arquivos e recursos fora do contrato; não recebe socket do runtime ou credenciais indevidas; engine ausente bloqueia; falha de isolamento não ativa execução no host.
- Evidência: verificações dos limites aplicados nas plataformas alvo e inventário de recursos antes/depois. Não confundir checkout separado com sandbox.

### [x] K-11 — Harness de verificação e evidências

- Dependências: K-05, K-10.
- Papéis: Plataforma/Harness e QA.
- Entrega: comandos de verificação por projeto, captura de status, artefatos e associação a critérios/versões; feedback estruturado de falhas.
- Aceitação: falha, skip e zero testes não viram sucesso indevido; logs truncados não eliminam o status real; evidência antiga não aprova conteúdo alterado; retorno não zero é preservado.
- Evidência: projeto-fixture com checks aprovados, reprovados, ignorados e resultados desatualizados.

### [x] K-12 — Fatia vertical de execução, QA e revisão

- Dependências: K-09, K-10, K-11.
- Papéis: Backend, Frontend, QA e Revisão.
- Entrega: uma tarefa aprovada percorre implementação, QA, revisão e correção limitada com modelo controlado.
- Aceitação: implementador não aprova a própria entrega; falha retorna com critério/evidência; revisão recebe contrato e resultado, não histórico irrestrito; limite de correções interrompe o loop.
- Evidência: demonstração TUI com primeira tentativa reprovada e segunda aceita para integração, ainda sem declarar tarefa concluída.

### [x] K-13 — Integração Git serializada e validação do conjunto

- Dependências: K-12.
- Papéis: Backend, Plataforma/Harness e Revisão.
- Entrega: branch de trabalho, integração, staging limitado, commits locais e checks do candidato integrado.
- Aceitação: branch original e alterações pré-existentes preservadas; conflito requer reconciliação e nova validação; falha de integração impede conclusão; nenhum push ou merge na principal é implícito.
- Evidência: fixtures Git com checkout sujo, arquivos não relacionados, conflitos, alteração da base e falha de check após integração.

### [x] K-14 — Scheduler paralelo e feedback do Orquestrador

- Dependências: K-13.
- Papéis: Backend e Plataforma/Harness.
- Entrega: concorrência limitada, dependências, claims, serialização de recursos e contratos de feedback.
- Aceitação: duas tarefas independentes progridem isoladas; dependente aguarda conclusão integrada; mesma tentativa não duplica; falha em um ramo só pausa outros quando há impacto; integração permanece serializada.
- Evidência: cenário com tarefas independentes, dependência transitiva, conflito e reinício durante despacho.

### [x] K-15 — Limites e contabilização

- Dependências: K-04, K-05, K-07.
- Papéis: Backend e Frontend.
- Entrega: limites concretos aprovados, consumo de planejamento e execução, reservas concorrentes quando mensuráveis e indicadores de custo.
- Aceitação: chamadas simultâneas não ignoram o orçamento disponível; indisponibilidade de preço/uso aparece explicitamente; tentativa de extrapolar impede novos despachos segundo a política; chamadas em voo são contabilizadas ou marcadas incertas.
- Evidência: cenários de quota esgotada, preço desconhecido, timeout e uso informado tardiamente. Nenhuma execução autônoma real será habilitada sem este gate.

### [x] K-16 — Pausa, cancelamento, retomada e reconciliação

- Dependências: K-14, K-15.
- Papéis: Backend, Frontend e Plataforma/Harness.
- Entrega: controles da TUI, checkpoints, reconciliação de processos/containers, chamadas pendentes e Git.
- Aceitação: pausa impede novo despacho; cancelamento distingue solicitação e confirmação; retomada não duplica commits ou comandos incertos; histórico e alterações do usuário sobrevivem à interrupção.
- Evidência: interrupções em chamadas, ferramentas, gravações e integração, com invariantes verificados após reinício.

### [ ] K-17 — Validação autorizada de provedores e plataformas

- Estado: testes controlados concluídos em Linux; provedor real, macOS e WSL2 não executados. Não há credencial/modelo/limite de uso remoto definidos para essa validação.

- Dependências: K-06, K-09, K-10, K-11, K-16.
- Papéis: Plataforma/Harness, QA e Revisão.
- Entrega: matriz de compatibilidade, testes do adapter com serviço real autorizado e verificação de Linux/macOS/WSL2.
- Aceitação: distinguir simulação e serviço real; mostrar limitações por endpoint; ausência de credencial bloqueia somente a validação dependente; reportar plataformas não executadas sem alegar suporte comprovado.
- Evidência: resultados identificados por versão, configuração sem segredos e plataforma; uso remoto apenas com autorização e limites. Não publicar nem usar tarefas reais do usuário como fixture.

### [ ] K-18 — Documentação, migração e aceite do MVP

- Estado: documentação e preservação do legado implementadas. Aceite completo aguarda K-17.

- Dependências: K-17.
- Papéis: Produto, Plataforma/Harness, QA e Revisão.
- Entrega: instalação, configuração, operação, recuperação, criação de papéis/skills, limitações e caminho explícito desde o runner antigo.
- Aceitação: critérios AC01–AC12 mapeados a evidências; nenhuma instrução nova depende de Claude/Codex CLI ou claude-code-template; comandos documentados reproduzíveis; nenhum resultado não executado é descrito como validado.
- Evidência: walkthrough em projeto-fixture, relatório de dependências e checklist de aceite. Remoção ou substituição do runner antigo somente conforme a estratégia de migração aprovada.

## Cobertura dos requisitos

| Requisitos | Tarefas principais |
| --- | --- |
| R01, R16 | K-07, K-12, K-15, K-16 |
| R02, R03, R04 | K-03, K-08, K-09 |
| R05, R06 | K-03, K-05, K-12, K-14, K-15 |
| R07, R08 | K-10, K-14, K-16 |
| R09 | K-11, K-12, K-13 |
| R10, R11 | K-04, K-05, K-17 |
| R12, R13 | K-06, K-08 |
| R14, R15 | K-03, K-08, K-13, K-16 |
| R17, R18 | K-01, K-02, K-07, K-10, K-17, K-18 |

Os itens marcados representam implementação com validação controlada, não comprovação de comportamento com qualquer modelo. K-17 e o aceite completo de K-18 continuam pendentes. A aprovação deste detalhamento foi concedida; gates operacionais e aprovações de planos de projetos gerenciados continuam obrigatórios.
