# ADR 0007 — Separar portas e publicar snapshots imutáveis

Status: aceito em 2026-11-06.

## Contexto

O novo pipeline terá estratégias substituíveis e múltiplas goroutines. Misturar
interfaces com concretos dificulta troca de implementação; compartilhar estado
mutável com a TUI cria risco de corrida.

## Decisão

Manter interfaces e implementações concretas em arquivos separados. Fazer casos
de uso dependerem de portas. Publicar para a TUI somente eventos e snapshots
profundamente clonados ou imutáveis.

## Consequências

- Mais tipos e arquivos de composição serão necessários.
- Testes de contrato poderão validar implementações alternativas.
- A TUI não pode modificar estado pertencente ao orquestrador.
