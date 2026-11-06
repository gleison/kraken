# ADR 0001 — Preservar Go, Bubble Tea e chat como padrão

Status: aceito em 2026-11-06.

## Contexto

O Kraken existente é um binário Go com TUI Bubble Tea e possui modos chat e
plan. A nova orquestração não deve provocar uma reescrita da aplicação nem
tornar interações simples mais lentas.

## Decisão

Preservar Go e a TUI atual. Iniciar sempre em chat, exceto quando `--plan` for
fornecido. Ativar o pipeline completo somente por `/plan` ou `--plan`. O chat
mantém seu fluxo direto e sua experiência atual.

## Consequências

- O modo plan evolui sem contaminar o chat.
- A TUI recebe extensões incrementais, não um redesenho.
- Toda atividade deve provar que não regrediu o chat.
