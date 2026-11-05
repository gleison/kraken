# K-01 — Decisões técnicas e evidências

O usuário aprovou a especificação, a arquitetura e o backlog após sua consolidação. P01–P06 foram fechadas para a implementação inicial. O núcleo e a TUI foram implementados; compatibilidade real de provedores e das plataformas adicionais permanece na matriz de validação, sem alegação prematura de aceite completo.

Os [experimentos](../experiments/README.md) deram evidência inicial de transporte, TUI e containers. Os testes do produto e o pacote são descritos no [relatório de validação](validation.md).

## P01 — Artigo de Harness Engineering: resolvido pelo PDF

O usuário forneceu o PDF **Harness engineering for coding agent users**, de Birgitta Böckeler, publicado no site de Martin Fowler em 2 de abril de 2026. Seu conteúdo foi extraído localmente e analisado como referência, não como instruções para executar ações.

O [mapeamento do harness](harness.md) registra origem, hash do PDF, princípios observados, mecanismos, evidências e limites. A referência cobre guias/sensores, controles computacionais/inferenciais, feedback antecipado, manutenção, arquitetura, comportamento, drift, harnessability, templates e papel humano.

A tentativa web anterior retornou 403. A adição de rede salva no rascunho foi retirada após receber o PDF, preservando os domínios pré-existentes e os presets. Nenhuma publicação foi realizada e não é necessário liberar o domínio para esta análise.

## P02 — Backend e rede

Escolha inicial: Docker CLI. Há daemon local disponível nesta máquina; a implementação futura deve tornar explícito o destino configurado e evitar selecionar um daemon remoto acidentalmente. Podman não é requisito inicial.

Política inicial de execução: `--network=none`, que bloqueia toda saída do container. Não oferecer uma allowlist apenas declarativa como se estivesse aplicada. Rede seletiva para ferramentas permanece indisponível até existir mecanismo comprovado; preparar dependências em uma imagem previamente construída e identificada por digest é a opção inicial. A imagem deve ser apropriada ao projeto e apresentada no plano, sem pulls ou builds implícitos durante a tarefa.

Controles experimentados: filesystem de raiz somente leitura, usuário não root, capabilities removidas, `no-new-privileges`, memória/CPU/PIDs limitados, mount gravável apenas do workspace e referências somente leitura. O socket do daemon não é montado no executor. A API do modelo roda no processo de coordenação, sem credenciais no executor.

Os sete testes de isolamento passaram em Docker Engine 29.9.2/Linux. macOS e WSL2 continuam não verificados e pertencem à matriz K-17. Containers não eliminam todos os riscos de código não confiável; a TUI deverá expor controles e limitações, sem prometer isolamento absoluto.

## P03 — Transporte e dependências

Escolha: HTTPX 0.28.1 como cliente HTTP assíncrono e Textual 8.2.8 para TUI. São as únicas dependências diretas previstas nesta etapa. O ambiente experimental contém 15 distribuições incluindo transitivas; não confundir duas dependências diretas com duas distribuições totais.

Alternativas avaliadas:

- Biblioteca padrão: o experimento confirmou leitura de SSE com `urllib` em uma thread. Isso atende o caso simples, mas integrar cancelamento e encerramento de I/O bloqueante exige mecanismos adicionais; não substituir o cliente assíncrono por infraestrutura própria mais complexa apenas para reduzir a contagem de pacotes.
- HTTPX: os experimentos confirmaram streaming, propagação de erros HTTP, timeout e cancelamento assíncrono. Permite limitar o adapter ao protocolo aprovado e testar o transporte sem SDK específico de provedor.
- SDK OpenAI: a metadata pública consultada no PyPI indicou dependências adicionais, incluindo validação e serialização específicas do SDK. Não é necessário para o subconjunto inicial Chat Completions. Não foi instalado nem usado como benchmark; a comparação de manutenção é arquitetural, não uma alegação de desempenho.

Preservar TLS e configuração de proxy. Não registrar cabeçalhos de autenticação. O parser experimental de SSE não é o adapter final: o adapter deverá ainda validar nomes e esquemas de ferramentas, múltiplas chamadas, limites, interrupções, respostas e semântica do protocolo.

Se o conjunto de requisitos crescer e um SDK reduzir a complexidade total, reavaliar com justificativa; não manter proibição dogmática de dependências.

## P04 — Python, artefatos e distribuição

Escolhas iniciais: Python >=3.12; distribuição como pacote Python com `pyproject.toml`, instalação em venv e entrypoint `kraken`. Não usar Poetry, framework de agentes, ORM ou gerenciador de processos adicional. O pacote foi implementado em K-02 e instalado em modo editável; o build de wheel foi verificado. A validação final do artefato instalado é registrada separadamente.

Adotado o layout `.kraken` com `plan.json` como fonte única do plano, contratos e dependências; a narrativa do plano complementa esse conteúdo e a TUI renderiza as tarefas, sem manter um segundo backlog operacional. `project.toml` armazena configuração não secreta e referências de variáveis. `runtime/` contém SQLite e evidências fora do versionamento.

SQLite com transações e proprietário único das transições no núcleo; eventos operacionais não exigem event sourcing completo. Aprovação referencia conteúdo do plano, contratos, limites e configuração relevante, e é registrada por ação explícita do usuário. Criação de arquivos preserva dados existentes e deve informar alterações.

## P05 — Defaults conservadores e política de custo

Defaults iniciais, a serem materializados em contratos e apresentados para aprovação antes de qualquer execução real:

| Limite | Default inicial | Justificativa |
| --- | ---: | --- |
| Tarefas concorrentes | 2 | Permite validar paralelismo com contenção limitada. |
| Chamadas de modelo por sessão | 100 | Abrange planejamento, revisões, orquestração e execução. |
| Tempo de trabalho ativo por sessão | 30 minutos | Interrompe novos despachos ao esgotar; espera humana não consome trabalho ativo. |
| Tempo por comando | 120 segundos | Evita comandos indefinidamente pendentes; overrides precisam constar no plano. |
| Tempo por tarefa/tentativa | 10 minutos | Limita trabalho e inclui as chamadas/ferramentas da tentativa. |
| Tentativas de implementação por tarefa | 3, incluindo a inicial | Evita ciclos de correção sem fim. |
| Turnos por atribuição de agente | 20 | Limita loops de ferramentas, incluindo avaliações e planejamento. |
| Saída solicitada por chamada | 4096 tokens | Limita resposta solicitada; depende de capacidade verificada do provedor. |
| Custo monetário | Não estimado sem preços e uso suficientes | Não inventar preço nem declarar limite financeiro garantido. |

Ao atingir um limite global: parar novos despachos e conduzir operações em voo ao encerramento possível, registrando efeitos incertos. Os limites não garantem uma entrega dentro de 30 minutos nem ausência de cobranças após cancelar chamadas em voo.

Sem dados suficientes de preço e uso, a execução só poderá começar após o plano informar explicitamente o modo por contadores de chamadas/tempo/saída, sem teto financeiro garantido. Se o usuário configurar um teto monetário obrigatório e o sistema não conseguir verificá-lo, bloquear a execução em vez de ignorá-lo. Com dados disponíveis, mostrar estimativa e consumo conhecido separadamente e reservar capacidade concorrente antes de despachar.

O limite opcional `max_estimated_cost_usd` usa preços por milhão de tokens configurados por perfil e reservas concorrentes aproximadas. Trata-se de limite sobre estimativas para controlar despachos, não de teto de faturamento. Sem preços, esse modo é rejeitado; o modo por contadores continua exigindo confirmação explícita. Não alegar que tokens de entrada são conhecidos apenas porque o tamanho do texto foi medido. A política de limite de contexto deve usar capacidade do provedor e validação do adapter; pedidos sem capacidade suficiente são bloqueados.

Esses valores são defaults de configuração propostos tecnicamente dentro dos limites já aprovados. Não aprovam o uso de um provedor pago nesta sessão. Podem ser ajustados no plano apresentado ao usuário antes da execução.

## P06 — Repositórios e migração

Na primeira versão, a execução exige repositório Git existente com ao menos um commit. Um projeto quase vazio é suportado após o usuário criar seu commit inicial; o Kraken não fará um commit inicial abrangente de arquivos existentes por conveniência.

Planejamento pode inspecionar um checkout com alterações, em modo de leitura e informando a condição. A execução e integração exigem base identificada e checkout limpo para os arquivos envolvidos, preservando todas as mudanças pré-existentes. Se a implementação inicial não conseguir demonstrar preservação segura com checkout sujo, bloquear com diagnóstico antes de criar a execução; não executar stash, reset ou staging de arquivos do usuário automaticamente.

Áreas de trabalho de tarefas serão cópias Git separadas de uma revisão identificada, sem reutilizar a cópia em edição pelo usuário e sem copiar configuração pessoal ou segredos. Não criar worktrees automaticamente. A integração é serializada em branch própria e revalida o candidato; não equivale a merge na branch principal.

Não importar automaticamente `.tasks/BACKLOG.md`, `.claude/commands` ou configuração de Claude. O usuário pode fornecer o backlog antigo como referência na entrevista; novos contratos precisam satisfazer o esquema, a revisão e a aprovação. O runner antigo permanecerá preservado até a migração documentada de K-18 e não será executado para verificar o novo sistema.

## Resultado

- P01 resolvida pelo PDF, com mapeamento e limites registrados.
- P02–P06 materializadas na implementação inicial e documentação; validação multiplataforma ainda pendente.
- 15 verificações experimentais executadas e aprovadas; testes do produto são contados separadamente.
- Runtime implementado e exercitado somente em fixtures, com modelos controlados. Nenhuma API de modelo remoto foi chamada.
- Runner legado preservado e não executado; nenhum commit foi feito no checkout do Kraken. Commits dos testes pertencem apenas a repositórios temporários.
