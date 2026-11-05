# Validação e aceite

## Versão 0.2.0 — fechamento das lacunas

A implementação foi validada nesta instância Linux com Python 3.12.14, Docker Engine 29.9.2, Textual 8.2.8 e HTTPX 0.28.1. Permanecem duas dependências diretas e 15 distribuições runtime em `requirements.lock`.

A rodada completa atual executou **82 testes: 82 aprovados, zero falhas, zero erros e zero skips**, em 34,430 segundos. Relatório JSON local: `/tmp/kraken-scope-results.json`. Os cenários usam modelos controlados; Docker e Git são reais, exclusivamente em projetos descartáveis. Não foi executada tarefa real do usuário nem chamada paga a modelo.

O mesmo conjunto de **82 testes passou contra o wheel 0.2.0 instalado em venv limpo**, em 37,600 segundos, importando `/tmp/kraken-v02-venv/lib/python3.12/site-packages/kraken`. O wheel inclui os seis manifestos de skills e tem SHA-256 `f534549cb6fd36d312f754f3e1fbfa301bf56c15a33918538efd259347345f49`.

Os 15 experimentos anteriores e os 46 testes da versão 0.1.0 são históricos, não somados à contagem atual. Um resultado verde de fixtures não certifica a qualidade de um modelo real.

## Evidências das lacunas

| Área | Verificação executada |
| --- | --- |
| Revisão/correção do plano | Achado técnico leva a correção e nova revisão; falta de intenção gera pergunta; campos bloqueados não mudam; limite de rodadas mantém plano não aprovado. |
| Orçamento pré-aprovação | Rascunho com limite maior não amplia o orçamento de planejamento configurado pelo usuário. |
| Replanejamento | Tarefa válida é revalidada e reaproveitada sem nova implementação; alteração invalida a tarefa e seus dependentes, preservando o independente. |
| Journal Git | Falhas em quatro fases, retomada sem novas chamadas de modelo, compare-and-swap, divergência recusada, evidência invalidada recusada e configuração nova exigindo novas avaliações. |
| Harness | unittest, pytest e JUnit; skips/xfails/disabled separados; relatórios ausentes ou contraditórios recusados; captura real de JUnit sem reutilizar arquivo anterior. |
| Skills | Origem/versão/requisitos no snapshot; runtime, rede e ferramenta incompatíveis recusados; variável ausente diagnosticada por nome; executável inexistente detectado na imagem aprovada. |
| Projetos/sessões | Seleção e criação pela TUI sem chamadas a API; aprovação, rascunho, consumo e histórico preservados ao retomar; lock impede duas sessões do mesmo projeto. |

Continuam cobertos os testes de streaming, tool calls inválidas, aprovação explícita, alteração de configuração, Docker sem rede, regressão na integração, checkout do usuário preservado, pausa, cancelamento e retomada. A [auditoria](scope-audit.md) relaciona os cenários aos itens do backlog.

A primeira tentativa dos novos testes TUI foi interrompida porque o sandbox de execução bloqueou o socket de despertar do event loop. A mesma suíte terminou com quatro aprovações em 3,823 segundos no executor com sockets locais liberados; a rodada completa usa esse executor. Isso não foi tratado como aprovação da execução interrompida nem contornado por remoção de asserts.

## Matriz externa

| Alvo | Estado nesta entrega |
| --- | --- |
| Linux, núcleo/TUI/Docker/Git | Executado localmente, com modelos controlados. |
| Núcleo/TUI em macOS | Workflow de runner hospedado configurado; resultado de CI deve ser consultado separadamente. |
| Integração Docker em macOS | Job manual para runner `self-hosted, macOS, kraken-docker`; não executado nesta instância. |
| WSL2 | Job manual para runner `self-hosted, linux, wsl2`, com verificação do kernel WSL2 real; não executado nesta instância. |
| Modelo real | Preferência OpenAI informada. Modelo, credencial e orçamento ainda não definidos; não executado. |

Workflows configurados não equivalem a resultados executados. Jobs opcionais sem runner não são testes aprovados. O core hospedado não tem Docker e reporta os skips de integração em JSON; o job Linux de integração exige zero skips.

## Reproduzir

```sh
python3.12 -m venv /tmp/kraken-tests
/tmp/kraken-tests/bin/python -m pip install -c requirements.lock .
docker --context default pull busybox@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec
/tmp/kraken-tests/bin/python scripts/run_tests.py --require-docker --report /tmp/test-results.json
```

Sem `--require-docker`, os testes Docker são ignorados explicitamente e aparecem no relatório. O runner preserva exit status e resultados individuais.

A CI em `.github/workflows/tests.yml` executa core/TUI em Ubuntu e macOS, integração em Ubuntu e disponibiliza jobs manuais para WSL2 e macOS com Docker. Para os jobs manuais, o runner precisa de Python 3.12 e daemon/contexto Docker `default` local, além das labels indicadas. WSL1 ou um Linux comum não satisfazem o gate WSL2.

## Aceite pendente

K-17 e o aceite final de K-18 exigem: configuração e autorização explícita da validação com modelo real, e evidências da matriz de plataformas correspondente. A ausência desses recursos não é descrita como código concluído e validado em todos os ambientes. Não há garantia de teto de faturamento nem de eliminar toda intervenção humana.
