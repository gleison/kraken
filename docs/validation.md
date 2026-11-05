# Validação da implementação inicial

## Ambiente e escopo

Validado nesta instância Linux com Python 3.12.14, Docker Engine 29.9.2, Textual 8.2.8 e HTTPX 0.28.1. Duas dependências diretas, 15 distribuições no conjunto runtime fixado em `requirements.lock`.

Os testes do produto usam modelos/transportes controlados. Os testes de integração executam comandos reais em containers e operações Git reais, exclusivamente em repositórios temporários. Não chamam modelos remotos nem executam `kraken.sh` ou tarefas deste projeto.

A rodada final executou **46 testes contra o wheel instalado: todos aprovados, zero reprovados e zero ignorados**, em 77,371 segundos. O código importado veio de `/tmp/kraken-installed-venv/lib/python3.12/site-packages/kraken`, não do checkout editável. Os **15 experimentos de K-01** são uma execução separada, não somados para inflar a contagem do produto.

## Evidências por critério

| Critério | Evidência executada | Limite |
| --- | --- | --- |
| AC01 — aprovação obrigatória | Aprovação explícita, revisão/configuração obsoletas, arquivo do plano alterado e gate antes de cada comando. | Validação local; aprovação não é assinatura criptográfica contra o próprio usuário. |
| AC02 — planejamento revisável | Entrevista, geração, revisores sem histórico, achado impeditivo e aprovação por modal. | Respostas dos modelos controladas; qualidade de entrevista com modelo real não medida. |
| AC03 — adapter | Tool calls intercaladas/fragmentadas, uso, JSON inválido, streaming incompleto, limite de saída e cancelamento sem retry. | Nenhum endpoint remoto foi certificado. |
| AC04 — paralelismo | Dois trabalhos isolados antes da conclusão do primeiro e dependente iniciado só após ambos concluírem. | Sem benchmark de escala; concorrência limitada. |
| AC05 — evidências | QA reprovado provoca correção; zero testes conhecido não aprova; regressão de tarefa anterior bloqueia integração. | Revisores controlados; resultados inferenciais reais permanecem probabilísticos. |
| AC06 — Git | Commits transferidos à branch local do Kraken sem mudar HEAD ou arquivos do checkout original; checkout sujo bloqueia. | Transferência interrompida exige reconciliação; nenhum push foi realizado. |
| AC07 — isolamento | Rede apenas loopback, filesystem restrito, fixture protegida, sem socket do daemon, capabilities removidas e escrita portátil dentro do workspace. | Testes Linux não provam isolamento absoluto nem suporte em qualquer engine. |
| AC08 — recuperação | Claim único, recuperação bloqueando efeitos incertos, cancelamento, reconciliação e retomada; pausa permite concluir trabalho em voo sem despachar dependente. | Não existe promessa de exatamente uma execução em serviços externos. |
| AC09 — limites e segredos | Reserva de chamadas/custo estimado, estimativa por preços configurados, redaction estrutural, configuração inválida, leitura de `.env` e symlink externo rejeitados. | Custo real não garantido; segredos versionados e comandos arbitrários precisam da política do projeto. |
| AC10 — TUI | Inicialização sem API/aprovação, modal de confirmação e terminal redimensionado; eventos de agentes/tarefas/logs vêm do runtime. | Sem avaliação de usabilidade humana extensa. |
| AC11 — perfis/skills | Overrides por papel, hash alterado invalidando configuração e skill dependente de runtime externo rejeitada. | Detecção básica não verifica a semântica de toda skill externa. |
| AC12 — plataformas | Instalação, importação, testes e Docker nesta instância Linux. | macOS e WSL2 não executados. |

O teste estrutural verifica que o núcleo não importa Textual. O exemplo comportamental usa bytes esperados definidos antes da implementação da fixture. Modelos simulados não são apresentados como QA ou revisão reais do código do Kraken.

## Empacotamento

Build de wheel e instalação editável foram executados. O artefato final foi construído em `/tmp/kraken-release-final/kraken_orchestrator-0.1.0-py3-none-any.whl`, com SHA-256 `531e218d496d61b6bf467c7c3f524b60e0d70cc1f5457c3c28154bdad068ecd0`.

O wheel foi instalado em venv limpo com as versões de `requirements.lock`; após a correção final, apenas o wheel foi reinstalado com dependências preservadas. A suíte final de 46 testes passou nesse ambiente. O arquivo zip do wheel passou na verificação de integridade e contém as seis skills padrão. `kraken --help` funcionou sem abrir TUI nem iniciar API. O teste de cancelamento durante spawn confirma encerramento do processo criado; a rodada final não apresentou o aviso de status de filho já recolhido observado antes da correção.

## Reproduzir os testes do produto

```sh
python3.12 -m venv /tmp/kraken-tests
/tmp/kraken-tests/bin/python -m pip install -c requirements.lock .
docker --context default pull busybox@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec
KRAKEN_TEST_DOCKER=1 /tmp/kraken-tests/bin/python -m unittest discover -s tests -v
```

Sem `KRAKEN_TEST_DOCKER=1`, os testes Docker são ignorados. Isso não demonstra prontidão do isolamento nem da integração. Não é necessário configurar API key para a suíte controlada.

## O que permanece pendente

- K-17: escolher endpoint, modelo e limites para uma validação remota explicitamente autorizada; credencial deve ser fornecida pelo mecanismo seguro do ambiente, nunca pelo chat ou pelos arquivos versionados.
- K-17: executar a matriz real de macOS e WSL2, incluindo contexto Docker e terminal.
- K-18: aceite completo após essa matriz, mantendo limites, resultados ignorados e problemas conhecidos separados.

A implementação local está disponível para revisão; o MVP não é declarado universalmente pronto. Nenhum commit foi feito no checkout do Kraken, nenhum push ou publicação foi realizado. Commits das fixtures existem apenas nos repositórios temporários e são descartados após os testes.
