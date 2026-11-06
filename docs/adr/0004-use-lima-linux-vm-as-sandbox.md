# ADR 0004 — Usar VM Linux gerenciada pelo Lima

Status: aceito em 2026-11-06.

## Contexto

Execução local não fornece isolamento forte e portátil. Docker foi rejeitado. O
produto suportará macOS e Linux, e pode instalar componentes auxiliares após
consentimento.

## Decisão

Usar Lima para gerenciar uma VM Linux em ambos os hosts. Executar toolchains,
comandos e código do projeto dentro dela. O Kraken faz bootstrap verificável de
Lima, imagem e guest tools. Não fornecer runner local na primeira versão.

## Consequências

- `go test` e outros toolchains nativos rodam no guest.
- Workloads exclusivos de macOS ficam fora do escopo inicial.
- A instalação gera artefatos externos versionados no diretório de dados.
- A disponibilidade de VZ/KVM passa a ser requisito operacional.
