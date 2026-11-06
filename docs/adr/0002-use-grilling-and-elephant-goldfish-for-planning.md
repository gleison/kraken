# ADR 0002 — Usar grilling e EGM no planejamento

Status: aceito em 2026-11-06.

## Contexto

Uma decomposição direta em tasks deixa lacunas de produto e mistura
planejamento, revisão e implementação. A referência EGM também é frequentemente
interpretada de forma incorreta como um modelo de subagentes implementadores.

## Decisão

Usar `grill-with-docs` em projetos e `grill-me` sem documentação persistente.
Aplicar EGM fielmente: Elephant mantém contexto; Goldfish são revisores
adversariais isolados. Agentes executores formam uma camada posterior e
separada. O planejamento requer aprovação versionada antes de qualquer escrita.

## Consequências

- Planejamento pode exigir várias interações antes da execução.
- Goldfish não implementam código.
- Skills e recursos precisam ser completos, versionados e auditáveis.
