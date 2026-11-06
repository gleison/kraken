# ADR 0008 — Vendorizar e revisar recursos upstream

Status: aceito em 2026-11-06.

## Contexto

Skills e modelos de referência podem mudar e possuem dependências entre arquivos.
Baixá-los em runtime tornaria o comportamento não reproduzível.

## Decisão

Vendorizar conteúdo completo com origem, commit, licença e hashes. Verificar
upstream semanalmente e abrir PR automática com diff e testes. Exigir revisão e
merge manuais. Sessões existentes continuam vinculadas à versão aprovada.

## Consequências

- O repositório inclui recursos de terceiros e respectivos avisos.
- Atualizações ficam visíveis e auditáveis.
- O CI normal valida integridade local sem falhar apenas por defasagem upstream.
