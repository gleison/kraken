# ADR 0005 — Persistir sessões fora do workspace

Status: aceito em 2026-11-06.

## Contexto

Planejamento aprovado, evidências e execução longa precisam sobreviver a
cancelamentos e reinícios sem poluir o repositório do usuário.

## Decisão

Definir uma porta de persistência e usar SQLite como implementação inicial.
Guardar banco e artefatos no diretório nativo do sistema, com override por
`KRAKEN_DATA_DIR`. Associar sessões à identidade do workspace e validar hashes
na retomada.

## Consequências

- Migrações e compatibilidade de schema tornam-se responsabilidades explícitas.
- Artefatos grandes ficam fora do banco.
- A persistência pode ser substituída sem alterar o domínio.
