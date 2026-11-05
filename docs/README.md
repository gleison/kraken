# Refatoração do Kraken

Status: base do MVP e detalhamento aprovados em conversa. Implementação autorizada conforme as dependências e os gates do backlog. K-01–K-16 incluem o fechamento das lacunas e testes específicos na versão 0.2.0. K-17 e o aceite final continuam pendentes de provedor real e plataformas adicionais.

- [Especificação do MVP](spec.md): requisitos, contratos, critérios de aceitação e lacunas.
- [Arquitetura proposta](architecture.md): componentes, persistência, fluxo e avaliação de dependências.
- [Backlog da refatoração](backlog.md): entregas incrementais, dependências e verificações esperadas.
- [Glossário](../GLOSSARY.md): linguagem compartilhada.
- [Harness baseado no artigo fornecido](harness.md): princípios, mecanismos e limites.
- [Uso e contratos](usage.md): formato do plano e recuperação.
- [Auditoria de fechamento de escopo](scope-audit.md): lacunas identificadas e seus testes.
- [Rechecagem requisito por requisito](requirements-audit.md): R01–R18, AC01–AC12 e limites das evidências.
- [Validação](validation.md): testes executados e o que permanece não verificado.
- [Decisões técnicas e evidências de K-01](technical-decisions.md): escolhas, experimentos e resolução da referência pelo PDF.

O `README.md` da raiz descreve a versão Python disponível. `kraken.sh` permanece como legado preservado. O aceite do MVP exige completar a matriz de validação, não apenas possuir uma implementação.

## Limites desta entrega

Na primeira entrega foram escritos apenas documentos. Após a aprovação e o recebimento do PDF, o núcleo e a TUI foram implementados e testados com modelos controlados, Docker e Git reais em projetos temporários. O desenvolvimento foi enviado ao GitHub no [PR #2](https://github.com/gleison/kraken/pull/2), conforme autorização do usuário. Não houve chamada a modelos remotos nem execução de tarefas reais do usuário. Esta rechecagem não publica o ambiente nem faz merge do PR.

O bloqueio de acesso ao artigo foi resolvido pelo PDF. As escolhas estão em [technical-decisions.md](technical-decisions.md); evidências atuais estão em [validation.md](validation.md). Não é necessário publicar o ambiente para ler a referência fornecida.
