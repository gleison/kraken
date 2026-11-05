# Contratos e operação

## Exemplo mínimo de plano

Este exemplo descreve uma fixture de demonstração, não um backlog para executar neste repositório. Ele pressupõe um projeto Git cujo commit já contém `expected.txt` com os bytes `hello\n`; seu hash deve ser calculado e inserido no campo `fixtures` antes da importação. Não invente esse hash nem aprove um exemplo sem revisar sua finalidade.

```json
{
  "objective": "Gerar uma saída que corresponda à fixture aprovada",
  "scope": "Apenas result.txt",
  "exclusions": "Sem rede, push, merge na principal ou deploy",
  "decisions": ["Usar a imagem local preparada e preservar os outros arquivos"],
  "glossary": {},
  "uncertainties": [],
  "fixtures": {"expected.txt": "SUBSTITUIR_PELO_SHA256_REAL_DO_ARQUIVO"},
  "image": "busybox@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec",
  "limits": {
    "concurrency": 2,
    "calls": 100,
    "active_seconds": 1800,
    "command_seconds": 120,
    "attempt_seconds": 600,
    "attempts": 3,
    "turns": 20,
    "output_tokens": 4096,
    "counter_only": true,
    "max_estimated_cost_usd": null
  },
  "tasks": [{
    "id": "TASK-1",
    "title": "Produzir saída aprovada",
    "goal": "Gerar result.txt com o conteúdo exato de expected.txt",
    "role": "backend",
    "allowed_paths": ["result.txt"],
    "depends_on": [],
    "references": ["expected.txt"],
    "criteria": [{"id": "C1", "description": "Saída corresponde aos bytes da fixture aprovada"}],
    "checks": [{
      "id": "CHECK-1",
      "argv": ["sh", "-c", "cmp result.txt expected.txt"],
      "criteria": ["C1"],
      "category": "behaviour",
      "timing": "before_integration"
    }]
  }]
}
```

O hash e a imagem são verificados. Critérios sem check, ciclos, referências de dependência ausentes e alterações permitidas em fixtures são rejeitados. Caminhos permitidos são arquivos ou diretórios explícitos, sem globs, traversal, metadados protegidos ou vírgulas. A raiz do projeto também não pode conter vírgula, devido ao formato de mounts do Docker.

O JSON é a fonte de autoridade das tarefas, limites e aprovações. `plan.md` é uma visualização. Alterações manuais em `plan.json`, perfis ou skills invalidam a aprovação até nova revisão. O Orquestrador confere os artefatos antes de despachar e antes de cada comando do agente.

## Ferramentas e imagem

A imagem aprovada precisa fornecer as ferramentas do projeto. O Kraken não presume que toda imagem contenha Python: escrita usa ferramentas POSIX e stdin; comandos de build, testes e navegador dependem da imagem escolhida. Falta de ferramenta produz falha e feedback, não instalação silenciosa ou fallback para a máquina.

Planejamento expõe somente leitura/listagem delimitadas do projeto, sem shell no host. Implementação recebe ferramentas dentro do container. QA e revisores recebem montagem de leitura e podem executar verificações com temporários em `/tmp`; checks que exigem gravar na árvore do projeto precisam ser adaptados para um local temporário antes de aprovar o plano.

Durante uma atribuição, o processo principal mantém as credenciais da API; não as injeta no executor. Metadados Git, metadados do Kraken e fixtures são montados somente para leitura quando presentes. Shells no container continuam podendo inspecionar os arquivos montados do projeto; não versionar segredos no repositório.

## Recuperação e preservação

- Tarefas concluídas e prontas da mesma revisão permanecem após fechar a TUI.
- Tentativa que estava executando, validando ou integrando vira bloqueada após uma interrupção não confirmada.
- **Reconciliar tarefa** verifica aprovação, limites, checkout e ausência de containers próprios pendentes antes de permitir nova tentativa isolada.
- Se uma transferência de commits já começou, o sistema bloqueia repetição automática. Confira a branch `kraken/<sessão>/<revisão>`, os workspaces de integração e as evidências. Não apagar o estado apenas para fazer o loop avançar.
- Evidências e candidatos ficam em `.kraken/runtime/`; não há limpeza automática agressiva de histórico ou workspaces.
- Não há comando de reset destrutivo nem atualização automática da branch principal. Para reiniciar uma sessão deliberadamente, primeiro confirme que não há trabalho em voo e preserve o diretório runtime fora do checkout; não use esse procedimento para ignorar operações incertas.

Um conflito/erro conhecido antes da transferência permanece em candidato isolado e pode gerar nova tentativa limitada. Uma falha durante a transferência pode ter alterado refs locais e exige reconciliação. A branch original não é sobrescrita por esses fluxos.

## Leituras e limites

A inspeção inicial fornece uma amostra limitada do projeto. O planejador pode ler outras referências por ferramentas controladas; arquivos fora do projeto, metadados e nomes potencialmente secretos são bloqueados. Essa classificação é preventiva e não substitui uma política de segredos.

A memória de decisões do planejamento e o histórico são persistentes. O histórico recente e a memória consolidada entram no contexto; limites de tamanho produzem diagnóstico explícito. Revisores do plano recebem artefatos, referências e uma rubrica de nove critérios, sem o histórico da entrevista.

O limite monetário opcional é **estimado** e depende de preços fornecidos na configuração. Reservas concorrentes reduzem excesso previsível, mas não garantem o valor faturado pelo provedor. Uso desconhecido permanece desconhecido; revisão de preço/configuração requer nova aprovação. Para um teto financeiro rígido, é necessário controle adicional no provedor; o Kraken não afirma fornecê-lo.
