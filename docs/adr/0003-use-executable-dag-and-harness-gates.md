# ADR 0003 — Usar DAG executável e gates de Harness

Status: aceito em 2026-11-06.

## Contexto

A lista atual contém apenas título e instrução e é executada sequencialmente. Ela
não expressa dependências reais, capacidades, critérios ou evidências.

## Decisão

Representar o plano como DAG de atividades contratuais. Cada atividade declara
feedforward, critérios, sensores, capacidades, evidências e orçamento. Aplicar
sensores de comportamento, arquitetura e manutenibilidade, computacionais antes
dos inferenciais. Começar com scheduler serial e permitir paralelismo seguro
posteriormente.

## Consequências

- O domínio e os eventos atuais precisarão evoluir.
- Conclusão depende de evidência, não do relato do agente.
- Falhas entram num ciclo limitado de correção.
