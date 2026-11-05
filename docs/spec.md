# Especificação do MVP

## Status e objetivo

A base funcional e este detalhamento foram aprovados pelo usuário em 5 de novembro de 2026. A implementação está autorizada conforme as dependências e os gates do backlog. Decisões técnicas e pendências continuam identificadas separadamente; ver [as decisões técnicas](technical-decisions.md) e o [relatório de validação](validation.md).

O Kraken será um orquestrador de desenvolvimento operado pelo terminal, com TUI, capaz de transformar uma intenção em plano aprovado, distribuir tarefas a agentes especializados e integrar resultados verificados. Deve reduzir intervenções humanas por meio de planejamento e contratos claros, sem esconder incertezas ou assumir autorizações ausentes.

Não será construído sobre os executáveis Claude Code ou Codex. Os modelos serão acessados diretamente por APIs.

## Situação atual

Antes da refatoração, o repositório continha apenas `README.md` e um runner Bash. O núcleo Python/TUI foi acrescentado após aprovação; o texto abaixo descreve a base legada. O runner antigo resolve o projeto a partir de sua própria localização, lê `.tasks/BACKLOG.md`, depende de templates em `.claude/commands` e invoca o executável `claude` em três fases sequenciais. Não contém TUI, backend próprio de modelos, agentes paralelos ou aprovação versionada do plano.

Após a fase de validação, o runner pode marcar a tarefa como concluída para fazer o loop avançar; também usa `git add -A`. O novo sistema deverá associar conclusão a evidências válidas e limitar o staging às alterações autorizadas. Essa observação resulta da leitura do código, sem executá-lo.

## Requisitos aprovados

| ID | Requisito |
| --- | --- |
| R01 | Ferramenta de terminal com TUI, indicadores baseados em atividades reais e acesso a logs. |
| R02 | Planejamento próprio inspirado em grill-me/grill-with-docs, sem depender do claude-code-template. |
| R03 | Aprovação humana explícita e obrigatória de uma versão consolidada do plano antes da implementação. |
| R04 | Revisão do plano por agentes com contexto novo antes de apresentá-lo para aprovação. |
| R05 | Orquestrador com contexto persistente e agentes especializados com contratos autossuficientes. |
| R06 | Execução autônoma dentro do plano, com limites configuráveis de tempo, custo e tentativas. |
| R07 | Tarefas independentes em paralelo, com concorrência configurável e ambientes separados. |
| R08 | Comandos dos agentes em containers, acessos limitados e sem fallback automático para execução direta na máquina. |
| R09 | Conclusão baseada em verificações e avaliações independentes, com evidências; o implementador não aprova o próprio trabalho. |
| R10 | API inicial Chat Completions compatível com OpenAI, com streaming e chamadas de ferramentas; capacidades verificadas antes da execução. |
| R11 | Endpoint, credenciais e modelo configuráveis por variáveis de ambiente; substituições opcionais por papel. |
| R12 | Perfis de papéis configuráveis e time proposto no planejamento, aprovado com o plano. |
| R13 | Skills em SKILL.md, com catálogo padrão, extensões por projeto, versões identificadas e avaliação de compatibilidade. |
| R14 | Planos, decisões e backlog locais e versionáveis; estado operacional separado, persistente e retomável. |
| R15 | Resultados em branch de trabalho do Kraken, commits locais automáticos e validação da integração. Push e merge na principal mediante autorização explícita. |
| R16 | TUI com pausa e cancelamento; decisões fora do escopo aprovado retornam ao usuário. |
| R17 | Linux e macOS; Windows via WSL2. Um repositório Git por projeto, incluindo monorepos. |
| R18 | Núcleo em Python, TUI em Textual, componentes separados e mínimo necessário de dependências. |

## Fora do MVP

- Coordenação de vários repositórios como um único projeto.
- Dependência obrigatória de GitHub Issues, Linear ou outro tracker.
- Interface web, serviço hospedado contínuo ou Windows nativo.
- Adapter da API Responses, sem impedir uma implementação posterior.
- Execução direta na máquina como fallback de containers indisponíveis.
- Push, merge na principal, deploy ou publicação implícitos.

Uma autorização futura para uma dessas operações deve identificar operação e destino; aprovação genérica do plano não a substitui. Interfaces de deploy e publicação não são entregas obrigatórias deste MVP.

## Planejamento e aprovação

O planejamento começa com inspeção somente de leitura do projeto e entrevista. Fatos verificáveis no código e nos documentos devem ser investigados pelo sistema, em vez de perguntados ao usuário. Decisões de produto e trade-offs pertencem ao usuário. A TUI deve permitir responder, corrigir e revisar decisões; a entrevista deve trabalhar as dependências entre decisões e evitar perguntas prematuras.

Artefatos mínimos do plano:

1. Problema, resultado esperado, usuários e cenários relevantes.
2. Escopo e exclusões; restrições e requisitos não funcionais aplicáveis.
3. Glossário e decisões, com justificativas quando houver trade-offs relevantes.
4. Contratos de tarefas e grafo de dependências sem ciclos.
5. Papéis, skills, modelos, ferramentas e capacidades necessários.
6. Ambiente de execução, recursos de rede e referências a credenciais necessárias.
7. Critérios de aceitação, comandos de verificação e evidências esperadas.
8. Limites de autonomia, concorrência, consumo e recuperação de falhas.
9. Incertezas restantes, impactos e condições de escalonamento.

Revisores com contexto novo verificam compreensão autossuficiente, simplicidade, coerência, dependências, critérios e prontidão. Recebem especificação e fontes pertinentes; não recebem a conversa anterior como substituto de documentação. Achados são classificados e resolvidos ou apresentados ao usuário. A aprovação final não pode ocultar lacunas impeditivas.

A TUI apresenta o plano consolidado e identifica sua versão. Apenas uma ação explícita do usuário registra aprovação. Silêncio, desconexão, retomada e sucesso de um modelo não aprovam o plano.

Mudanças em escopo, decisões, contratos, capacidades ou limites aprovados pausam o trabalho afetado e exigem nova aprovação. Resultados antigos só podem ser reutilizados após verificar compatibilidade com a revisão. Ajustes de implementação que respeitam o contrato não exigem nova aprovação. Uma mudança compartilhada deve considerar também os dependentes afetados.

## Contratos de comunicação

### Antes de uma tarefa: feedforward

| Campo | Conteúdo mínimo |
| --- | --- |
| Identidade | Projeto, tarefa, tentativa e versão aprovada do plano. |
| Objetivo | Resultado observável e motivo pelo qual ele é necessário. |
| Escopo | Alterações permitidas, exclusões e limites de responsabilidade. |
| Referências | Requisitos, decisões, código, interfaces e artefatos necessários. |
| Dependências | Pré-requisitos e versões dos resultados consumidos. |
| Execução | Papel, skills, modelo, ferramentas, ambiente e recursos autorizados. |
| Aceitação | Critérios, verificações, evidências e avaliadores. |
| Limites | Tempo, consumo, tentativas e condições de escalonamento. |
| Saída | Formato do relatório, identificação das alterações e bloqueios. |

O contexto deve ser suficiente e conciso, com referências resolvíveis. Uma conversa inteira não deve ser enviada por padrão. Memória persistente não significa uma janela de contexto ilimitada.

### Após uma tentativa: resultado e feedback

O agente entrega resumo, alterações, verificações executadas, evidências, desvios, consumo conhecido, riscos e bloqueios. O Orquestrador verifica esses dados e devolve: critério violado, resultado esperado, observação concreta, referência à evidência e condição para a próxima tentativa. Feedback genérico como “melhore o código” não constitui um contrato de correção.

Saídas de modelos e ferramentas são dados não confiáveis para fins de autorização. Não podem aprovar planos, ampliar permissões ou remover gates. O sistema rejeita estruturas inválidas e registra a falha, sem inferir sucesso de texto livre.

## Execução, isolamento e integração

O Orquestrador seleciona tarefas com dependências satisfeitas e capacidade disponível. Não despacha a mesma tentativa duas vezes; operações e resultados têm identificadores estáveis.

Cada tarefa tem área de trabalho e container próprios. Independência no grafo é necessária, mas também é preciso avaliar interfaces e conflitos previsíveis. Um recurso compartilhado que não pode ser isolado exige serialização ou bloqueio explícito.

Os containers recebem apenas os mounts e recursos necessários. Credenciais de provedores ficam preferencialmente no processo responsável pelas APIs, fora dos containers executores. Não montar o socket do runtime nos containers nem disponibilizar diretórios pessoais completos. Rede deve respeitar uma política efetivamente aplicada; uma declaração no plano, sozinha, não implementa isolamento. Não anunciar containers como garantia absoluta de segurança.

Somente o componente responsável pela integração altera a branch compartilhada do Kraken. O staging inclui apenas o resultado validado; alterações pré-existentes do usuário devem ser preservadas. Evidências são vinculadas à versão avaliada. Se a integração muda o resultado ou cria conflitos, as verificações pertinentes são repetidas. Uma tarefa só é concluída após integração e aceitação válidas para essa versão.

Após QA ou revisão reprovados, uma nova tentativa recebe feedback concreto. O histórico anterior permanece consultável. Repetições consomem os limites aprovados e não podem continuar indefinidamente.

## Time e skills

| Papel | Responsabilidade e independência | Composição proposta |
| --- | --- | --- |
| Analista de Produto | Descoberta, escopo, critérios e backlog; não aprova em nome do usuário. | Procedimentos inspirados em grill-with-docs e contratos próprios. |
| Backend | Serviços, contratos e regras de negócio. | Perfil próprio, convenções do projeto e procedimentos de testes. |
| Frontend | Interface, estados, interação e acessibilidade. | Perfil próprio e ferramentas de navegador quando necessárias. |
| QA | Verificar critérios e cenários relevantes, com evidências independentes. | Perfil próprio, fixtures e procedimentos de teste adequados ao projeto. |
| Revisor de código | Aderência ao plano, qualidade e riscos do diff. | Contexto novo e rubricas explícitas. |
| Plataforma/Harness | Ambiente reproduzível, verificações, observabilidade e controles. | Capacidade recomendada desde o início; agente dedicado conforme necessidade. |
| UX ou Segurança | Avaliações especializadas conforme o projeto. | Perfis opcionais justificados no planejamento. |

Independência exige atribuições e contextos separados e verificações externas ao relato do implementador. Não exige provedores diferentes, nem pressupõe ausência de erros correlacionados entre modelos. O rigor deve ser proporcional ao trabalho, preservando avaliação independente.

Uma skill fornece procedimento; o perfil define responsabilidades, recursos, entradas e saídas. Skills não concedem permissões. Dependências de ferramentas, scripts ou convenções de um CLI externo devem ser declaradas e adaptadas. A versão aprovada de uma skill deve ser identificável, sem atualização silenciosa durante a execução. Respeitar licenças e atribuição de materiais reutilizados; não copiar o claude-code-template.

## TUI e operação

A TUI deve permitir acompanhar planejamento, aprovação, execução, bloqueios, avaliação e integração. Deve exibir tarefas, dependências, agentes ativos, ferramentas/comandos em andamento, logs sanitizados e consumo conhecido. Estados e contadores vêm de eventos do runtime; não apresentar porcentagem estimada como progresso medido.

Pausa impede novos despachos e permite conduzir trabalho em andamento a um ponto seguro. Cancelamento solicita interrupção das operações controladas pelo Kraken e preserva o histórico. Ambos devem distinguir pedido recebido de interrupção confirmada. Interromper uma chamada de API pode não impedir consumo no provedor; comunicar operações cujo resultado ainda é desconhecido.

Após reinício, reconciliar processos, containers, chamadas e estado Git antes de retomar. Nunca repetir cegamente uma operação com efeitos cuja conclusão seja incerta. Falta de resposta do usuário mantém a decisão pendente; não autoriza continuidade dependente dessa decisão.

Cor não será o único indicador de estado. Painéis devem continuar utilizáveis com terminal pequeno, saída extensa e eventos frequentes. Logs não devem expor segredos, e sua captura terá limites de volume e retenção.

## Configuração de modelos e limites

A nomenclatura exata é proposta na arquitetura. A configuração deve separar endpoint, modelo e referência à variável que contém a credencial. Cada papel herda a configuração padrão, podendo substituí-la. Não gravar valores secretos no plano, no backlog, no estado ou nos logs.

Compatibilidade inicial significa Chat Completions com streaming e chamadas de ferramentas, incluindo reconstrução e validação dos argumentos transmitidos em partes. Respostas inválidas, credenciais ausentes, capacidades insuficientes e limites de contexto produzem diagnósticos explícitos. Não selecionar outro provedor ou protocolo silenciosamente.

Limites devem abranger execução e planejamento; a entrevista e suas revisões também consomem recursos. Uma verificação remota de capacidades pode gerar custo e precisa de autorização informada para esse fim, sem autorização implícita para implementar.

Custos só podem ser calculados quando houver uso e preços conhecidos. Na ausência dessas informações, mostrar “indisponível” ou “estimativa” e manter limites verificáveis de tempo, chamadas, tokens quando controláveis e tentativas. Não prometer um teto financeiro exato sem medição e controle suficientes. Definir a política aplicável quando o limite financeiro não puder ser verificado antes de executar com essa configuração.

## Critérios de aceitação do MVP

| ID | Demonstração necessária |
| --- | --- |
| AC01 | Nenhuma ferramenta de implementação executa sem aprovação válida, inclusive após reinício ou alteração do plano. |
| AC02 | Entrevista produz plano com contratos, dependências, time e critérios; revisão com contexto novo identifica uma lacuna sem acessar o histórico. |
| AC03 | Adapter reconstrói tool calls em streaming; dados inválidos e ferramentas não autorizadas não são executados. |
| AC04 | Duas tarefas independentes rodam isoladas; dependente aguarda pré-requisito; não há despacho duplicado. |
| AC05 | QA reprovado impede conclusão, gera feedback e correção limitada; declaração de sucesso do modelo não substitui evidências. |
| AC06 | Integração preserva mudanças do usuário, limita staging e revalida o conjunto; não ocorre push ou merge na principal sem autorização. |
| AC07 | Containers indisponíveis ou isolamento insuficiente impedem a execução afetada, sem fallback para o host. |
| AC08 | Pausa, cancelamento e retomada preservam estado e reconciliam operações incertas, sem repetir efeitos cegamente. |
| AC09 | Limites param novos despachos; custos desconhecidos são apresentados honestamente; segredos não aparecem nos registros. |
| AC10 | TUI acompanha eventos reais, continua responsiva e permite responder às decisões e inspecionar evidências. |
| AC11 | Configuração por papel funciona; skill incompatível ou alterada é detectada antes de seu uso autorizado. |
| AC12 | Fluxo reproduzível é validado em Linux, macOS e WSL2, com pré-requisitos e limites conhecidos documentados. |

Os testes devem distinguir resultados aprovados, reprovados, ignorados e não executados. Testes com provedores simulados validam o runtime, mas não comprovam compatibilidade com serviços reais. Validação remota futura exige configuração e autorização próprias; não integra esta entrega documental.

## Pendencias antes da implementacao

| ID | Pendência | Encaminhamento proposto |
| --- | --- | --- |
| P01 | Confrontar o harness com o artigo indicado de Martin Fowler, cujo acesso foi bloqueado na pesquisa. | Ler a fonte quando acessível ou receber cópia fornecida pelo usuário; registrar princípios, mapeamento e diferenças antes de fechar o desenho do harness. |
| P02 | Backend inicial de containers e política de rede efetivamente aplicada nas plataformas alvo. | Avaliar Docker/Podman, disponibilidade, mounts, cancelamento e aplicação de egress; escolher e documentar limitações. |
| P03 | Transporte de API e dependências adicionais. | Comparar biblioteca padrão, cliente HTTP e SDK com spike de streaming, ferramentas, cancelamento e timeouts. |
| P04 | Formato e localização definitivos dos artefatos, versão mínima de Python e distribuição. | Revisar as propostas da arquitetura e demonstrar instalação em ambientes limpos. |
| P05 | Defaults de concorrência, tempo, tokens, chamadas, tentativas e política de custo indisponível. | Propor valores conservadores com justificativa; apresentar os limites concretos no plano antes de executar. |
| P06 | Fluxo de projetos vazios, importação do backlog antigo e política detalhada de Git com checkout sujo. | Propor cenários suportados para o MVP, preservação e bloqueios; não presumir migração automática. |

P01–P06 foram resolvidas para a implementação inicial conforme [technical-decisions.md](technical-decisions.md). A tabela preserva as questões que orientaram K-01. A validação com provedor real e a matriz multiplataforma permanecem pendentes, sem invalidar a autorização já concedida para implementar e testar com fixtures. Descobertas que mudem escopo continuam exigindo retorno pontual ao usuário.
