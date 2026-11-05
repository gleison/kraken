# Harness do Kraken

Fonte: **Harness engineering for coding agent users**, de **Birgitta Böckeler**, publicado no site de Martin Fowler em 2 de abril de 2026. URL: <https://martinfowler.com/articles/harness-engineering.html>.

A análise foi feita a partir do PDF fornecido pelo usuário, não de uma consulta web bem-sucedida. SHA-256 do PDF: `979816be9264f198b594cf2da5ecb9583fd67ace88b2a0047784b6cc9b223f07`. Não é necessário liberar o domínio para esta etapa; a adição temporária foi retirada do rascunho de rede. O artigo não foi incorporado integralmente ao repositório.

## Princípios e mecanismos

| Princípio observado na fonte | Aplicação no Kraken | Evidência ou limite |
| --- | --- | --- |
| Guias antes da ação e sensores após a ação se complementam. | Contratos, decisões, skills e feedforward precedem implementação; checks, QA e revisão produzem feedback. | `session.py`, `agent.py` e `orchestrator.py`; teste com QA reprovado e correção. |
| Controles computacionais são rápidos e determinísticos; inferenciais acrescentam julgamento sem garantia determinística. | Código aplica aprovação, permissões, limites, dependências e Git; modelos planejam, orientam e revisam. | Testes rejeitam aprovação ausente, revisão obsoleta, tool calls inválidas e despacho duplicado. |
| Feedback rápido deve ocorrer antes da integração; controles mais caros podem ocorrer depois. | Checks `before_integration`, repetidos no candidato integrado; checks adicionais `after_integration`. | Integração reexecuta também checks de tarefas já concluídas, detectando regressões. |
| Manutenibilidade, adequação arquitetural e comportamento exigem controles distintos. | Checks declaram `maintainability`, `architecture` ou `behaviour`; cada tarefa exige check comportamental antes da integração. | O teste estrutural impede dependência do núcleo em Textual. Cobertura adequada de cada categoria depende do projeto. |
| Uma suíte gerada pelo agente não demonstra sozinha que ele entendeu a intenção. | Critérios aprovados, fixtures protegidas por hash, revisão do plano e QA com contexto novo. | Testes usam saída esperada previamente definida. Detectamos zero testes nos formatos conhecidos, sem alegar interpretar qualquer runner. |
| Falhas repetidas devem orientar melhorias no harness. | Histórico de tentativas, feedback vinculado a evidências e sugestões de revisão de controles. | Mudanças de contratos, fixtures, skills e limites exigem revisão/aprovação; o sistema não enfraquece sensores automaticamente. |
| Drift pode exigir sensores fora do ciclo de uma mudança. | Checks opcionais `health`, executados explicitamente por `kraken health`. | Não há daemon, monitoramento de produção ou verificação contínua automática neste MVP. |
| Harnessability depende das propriedades do projeto. | Planejamento inspeciona código e ambiente; imagem, comandos e critérios são específicos do projeto. | Código legado pode exigir preparação adicional ou escalonamento. Não prometer o mesmo grau de autonomia em qualquer base. |
| Templates ajudam a restringir o espaço de possibilidades, mas podem divergir com o tempo. | Catálogo pequeno de skills, extensões locais e hashes de conteúdo no snapshot aprovado. | Atualização de uma skill invalida a aprovação; compatibilidade básica não prova adequação semântica. |
| O papel humano deve se concentrar nas decisões mais importantes, não desaparecer. | Entrevista, aprovação obrigatória e escalonamento quando o contrato não resolve a decisão. | Silêncio e sucesso de um modelo não concedem aprovação. Avaliação por IA continua falível. |

## Onde cada controle roda

- **Planejamento:** inspeção limitada de leitura, entrevista, memória de decisões, contratos, exemplos e revisores independentes. Não existem ferramentas de escrita ou shell no host para o planejador.
- **Antes de cada comando:** conferir a aprovação atual, plano em disco, configuração, skills e limites. O executor sem política explícita recusa a operação.
- **Implementação:** container sem rede, ferramentas delimitadas, contexto da tarefa e orientação do Orquestrador. Código não relacionado não entra no commit.
- **Antes da integração:** checks comportamentais e demais checks rápidos; QA e revisão independentes avaliam a versão identificada.
- **Candidato integrado:** repetir controles e verificar também tarefas já concluídas; só depois transferir commits para a branch local do Kraken.
- **Saúde:** executar sensores aprovados sob demanda, com evidências vinculadas ao commit inspecionado. Ausência de checks é reportada, não considerada aprovação.
- **Recuperação:** distinguir resultado conhecido de efeito incerto. Uma transferência Git interrompida requer reconciliação antes de repetir.

## Limites reconhecidos

Uma suíte verde pode testar a coisa errada. Modelos distintos podem cometer o mesmo erro. Um conjunto de checks silencioso pode significar boa qualidade ou pouca capacidade de detecção. Categorizar sensores não comprova cobertura suficiente.

Por isso, a aprovação humana do plano continua obrigatória. Fixtures aprovadas são úteis quando aplicáveis, não uma solução universal. O runtime fornece mecanismos e evidências; a qualidade dos critérios e a adequação dos sensores precisam ser avaliadas em cada projeto.
