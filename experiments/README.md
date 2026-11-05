# Experimentos técnicos de K-01

Estes arquivos verificam escolhas de bibliotecas e isolamento. Não são a implementação do Kraken, não chamam provedores de modelos e não executam tarefas do projeto. Não importam nem executam `kraken.sh`.

Ambiente observado: Python 3.12.14, Textual 8.2.8, HTTPX 0.28.1 e Docker Engine 29.9.2 em Linux. `requirements.txt` fixa as versões diretas e transitivas do ambiente experimental; ainda não é o manifesto de distribuição do produto.

## Reprodução

```sh
python3.12 -m venv /tmp/kraken-k01-venv
/tmp/kraken-k01-venv/bin/python -m pip install -r experiments/requirements.txt
/tmp/kraken-k01-venv/bin/python -m unittest discover -s experiments -p test_transport.py -v
/tmp/kraken-k01-venv/bin/python -m unittest discover -s experiments -p test_textual.py -v
```

Os testes HTTP usam um servidor de fixture em loopback. Não precisa de chave, endpoint externo ou modelo. Eles verificam o transporte e um parser mínimo da fixture, não um adapter de produção completo nem compatibilidade universal com Chat Completions.

O teste Textual usa a API pública `run_test`; não abre preview nem sessão de produto.

## Docker

Os testes de container usam explicitamente o daemon local Linux, não contextos remotos herdados. Não executar esse arquivo em uma máquina cujo socket local não corresponda ao ambiente pretendido. Docker Desktop/macOS e WSL2 ainda precisam de validação própria.

```sh
env -u DOCKER_HOST -u DOCKER_CONTEXT -u DOCKER_TLS \
    -u DOCKER_TLS_VERIFY -u DOCKER_CERT_PATH \
    docker --host=unix:///var/run/docker.sock pull \
    busybox@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec
/tmp/kraken-k01-venv/bin/python -m unittest discover -s experiments -p test_container.py -v
```

A imagem foi obtida do registro oficial com checksum verificado pelo Docker e os testes a referenciam por digest. Eles criam diretórios temporários próprios e containers descartáveis, sem montar o repositório, credenciais ou socket Docker. Não removem imagens nem containers de outros trabalhos.

O ambiente experimental requer duas dependências diretas: Textual e HTTPX, totalizando 15 distribuições instaladas com suas dependências transitivas. SDK de provedor e framework de agentes não foram instalados.

## Resultado desta execução

| Grupo | Executados | Aprovados | Reprovados | Ignorados |
| --- | ---: | ---: | ---: | ---: |
| Transporte HTTP local | 6 | 6 | 0 | 0 |
| TUI headless | 2 | 2 | 0 | 0 |
| Containers Linux | 7 | 7 | 0 | 0 |

As verificações de containers cobrem interface de rede apenas loopback, escrita no mount autorizado, referência somente leitura, tentativa de escrita negada na raiz, ausência do socket Docker, capabilities removidas e remoção de container após execução.

Não validado: agente real, API de modelo real, controle financeiro real, autonomia de implementação, integração Git do futuro runtime, macOS ou WSL2. Não inferir desses experimentos que o MVP está implementado ou pronto.
