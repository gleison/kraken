# Glossário do Kraken 2.0

Este glossário define os termos usados na especificação e no backlog.

## Activity

Unidade executável do plano. Possui dependências, papel, escopo, critérios,
sensores, capacidades, evidências e orçamento. Substitui a `Task` simples atual.

## Aprovação versionada

Autorização humana vinculada aos hashes exatos do plano, configuração, skills,
workspace e runtime. Mudança material exige nova aprovação.

## Artefato

Arquivo ou conjunto de dados produzido pelo planejamento, executor ou sensor e
referenciado pela sessão, como diff, log, relatório, ADR ou resultado de teste.

## Capability

Permissão explícita para acessar um recurso ou executar uma classe de operação,
como escrever no worktree, usar rede na fase de dependências ou invocar uma
ferramenta.

## DAG

Grafo direcionado sem ciclos que representa atividades e dependências. Uma
lista sequencial é um caso particular de DAG.

## Elephant

Agente que mantém o contexto do planejamento, elabora a especificação e resolve
findings. Não é sinônimo do orquestrador de execução.

## Evidence

Prova verificável de que um critério ou sensor foi atendido: código de saída,
relatório, diff, artefato, hash ou finding resolvido.

## Feedforward / Guide

Contexto e orientação entregues antes da ação: objetivo, decisões, escopo,
restrições, ferramentas e definição de pronto.

## Feedback / Sensor

Sinal produzido depois da ação para verificar ou corrigir o resultado. Pode ser
computacional, como teste e linter, ou inferencial, como QA e code review.

## Finding

Problema estruturado identificado por um Goldfish, QA ou Revisor, contendo
severidade, localização, discrepância e justificativa.

## Frontier

Conjunto de decisões da árvore de planejamento cujas premissas já foram
resolvidas e que podem ser perguntadas na rodada atual.

## Goldfish

Revisor adversarial, stateless e de contexto fresco usado no planejamento EGM.
Não implementa atividades.

## Harness

Sistema de guides, sensors, gates, evidências e loops de correção que aumenta a
chance de acerto e permite autocorreção antes da revisão humana.

## Persona / Role profile

Configuração stateless de um papel contendo missão, limites, skills,
ferramentas, rubrica, modelo e formato de saída.

## Plan revision

Versão imutável de uma especificação e seu DAG. É a unidade apresentada para
aprovação.

## Runner

Porta usada para executar operações concretas em um ambiente isolado. A
implementação inicial de produção será baseada em Lima.

## Sensor computacional

Verificação determinística, como formatação, build, teste, linter ou análise
estrutural.

## Sensor inferencial

Avaliação semântica e probabilística feita por um agente, como QA, revisão de
arquitetura ou LLM-as-judge.

## Session

Agregado persistente que contém planejamento, aprovação, execução, eventos,
evidências, orçamento e referências ao workspace.

## Skill

Procedimento declarativo versionado composto por instruções, dependências,
recursos e schemas. Não é executado como código e não é sinônimo de persona.

## Snapshot

Cópia imutável do estado publicada pelo orquestrador para a TUI, evitando estado
mutável compartilhado entre goroutines.

## Worktree de atividade

Diretório Git isolado no qual um executor modifica arquivos e produz commits
sem tocar diretamente na branch original.
