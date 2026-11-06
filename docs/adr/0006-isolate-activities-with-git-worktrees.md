# ADR 0006 — Isolar atividades com Git worktrees

Status: aceito em 2026-11-06.

## Contexto

Agentes autônomos e futuro paralelismo não devem modificar simultaneamente o
workspace principal nem sobrescrever trabalho do usuário.

## Decisão

Criar branch e worktree por atividade. Integrar commits numa branch de sessão e
só atualizar a branch original depois de todos os gates, se ela permanecer
limpa e no commit-base aprovado.

## Consequências

- Cada atividade produz uma unidade rastreável.
- Conflitos são tratados fora da branch original.
- Projetos sem Git precisarão de tratamento futuro; não há fallback implícito.
