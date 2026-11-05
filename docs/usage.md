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

O time exibido deriva dos papéis das tarefas, com Produto, Orquestrador, QA e Revisor como participantes obrigatórios. O planejamento recebe os perfis e capacidades configurados para propor as especialidades necessárias. O modal de aprovação apresenta modelos, ferramentas, skills e requisitos junto do plano.

Tarefas sem dependência podem executar em paralelo quando seus caminhos de edição não se sobrepõem. Arquivos iguais ou diretórios ancestrais são serializados. O scheduler continua procurando outras tarefas independentes enquanto esse recurso está ocupado.

## Ferramentas e imagem

A imagem aprovada precisa fornecer as ferramentas do projeto. O Kraken não presume que toda imagem contenha Python: escrita usa ferramentas POSIX e stdin; comandos de build, testes e navegador dependem da imagem escolhida. Falta de ferramenta produz falha e feedback, não instalação silenciosa ou fallback para a máquina.

Execute o Kraken com usuário não root e acesso ao contexto Docker local. O executor reutiliza UID/GID desse usuário e recusa UID zero antes de iniciar containers.

Planejamento expõe somente leitura/listagem delimitadas do projeto, sem shell no host. Implementação recebe ferramentas dentro do container. QA e revisores recebem montagem de leitura e podem executar verificações com temporários em `/tmp`; checks que exigem gravar na árvore do projeto precisam ser adaptados para um local temporário antes de aprovar o plano.

Durante uma atribuição, o processo principal mantém as credenciais da API; não as injeta no executor. Metadados Git, metadados do Kraken e fixtures são montados somente para leitura quando presentes. Shells no container continuam podendo inspecionar os arquivos montados do projeto; não versionar segredos no repositório.

## Recuperação e preservação

- Tarefas concluídas e prontas da mesma revisão permanecem após fechar a TUI.
- Tentativa que estava executando, validando ou integrando vira bloqueada após uma interrupção não confirmada.
- **Reconciliar tarefa** verifica aprovação, limites, checkout e ausência de containers próprios pendentes antes de permitir nova tentativa isolada.
- Se uma transferência já começou, a retomada verifica o journal, o commit candidato, a base, a configuração aprovada e todas as evidências. Completa somente as fases faltantes e não repete implementação. Ref divergente, falta de provas ou journal legado incompleto exigem inspeção. Não apagar o estado para fazer o loop avançar.
- Evidências e candidatos ficam em `.kraken/runtime/`; não há limpeza automática agressiva de histórico ou workspaces.
- Não há comando de reset destrutivo nem atualização automática da branch principal. Para reiniciar uma sessão deliberadamente, primeiro confirme que não há trabalho em voo e preserve o diretório runtime fora do checkout; não use esse procedimento para ignorar operações incertas.

Um conflito/erro conhecido antes da transferência permanece em candidato isolado e pode gerar nova tentativa limitada. Uma falha durante a transferência pode ter alterado refs locais; o journal permite reconciliação idempotente quando não há divergência. A branch original não é sobrescrita por esses fluxos.

## Leituras e limites

A inspeção inicial fornece uma amostra limitada do projeto. O planejador pode ler outras referências por ferramentas controladas; arquivos fora do projeto, metadados e nomes potencialmente secretos são bloqueados. Essa classificação é preventiva e não substitui uma política de segredos.

A memória de decisões do planejamento e o histórico são persistentes. O histórico recente e a memória consolidada entram no contexto; limites de tamanho produzem diagnóstico explícito. Revisores do plano recebem artefatos, referências e uma rubrica de nove critérios, sem o histórico da entrevista.

O limite monetário opcional é **estimado** e depende de preços fornecidos na configuração. Reservas concorrentes reduzem excesso previsível, mas não garantem o valor faturado pelo provedor. Uso desconhecido permanece desconhecido; revisão de preço/configuração requer nova aprovação. Para um teto financeiro rígido, é necessário controle adicional no provedor; o Kraken não afirma fornecê-lo.


## Sessões pela TUI

`kraken tui` abre o catálogo. Informe uma raiz Git, abra o projeto, escolha uma sessão existente ou crie uma nova. `Projetos/sessões` permite a mesma seleção depois de encerrar uma operação em voo. Inicializar/selecionar um projeto não chama API.

O catálogo fica em `$XDG_STATE_HOME/kraken` (ou `~/.local/state/kraken`). Use `kraken tui --state-dir /caminho/do/catalogo` para escolher outro local. Cada projeto mantém seus dados em `.kraken/runtime/sessions/<id>`. Sessões do layout antigo continuam na localização legada, sem mover workspaces/journals. Um lock do projeto impede duas sessões simultâneas editando o mesmo plano de apresentação.

`plan.json`/`plan.md` mostram a sessão ativa. Antes de trocar, seus rascunhos são preservados na sessão anterior; retomar restaura a apresentação correspondente, inclusive edições manuais. Configuração de provedores/skills continua sendo do projeto: alterações invalidam aprovações anteriores. O orçamento e histórico de cada sessão são preservados.

Na aba **Tarefas**, a coluna de dependências mostra os pré-requisitos. **Inspecionar evidências da tarefa** lista até 100 registros persistidos, dos mais recentes aos mais antigos; selecione um para ver critérios, versão, resultado e saída. A consulta independe do log de eventos. Conteúdos acima de 60 mil caracteres são sinalizados como visualização parcial; o registro integral permanece no SQLite da sessão.

## Manifesto de skill

Ao lado de `SKILL.md`, pode existir `skill.toml`:

```toml
source = "https://github.com/organizacao/skills"
version = "revisao-fixada"
license = "MIT"
runtime = "kraken"
required_tools = ["read_file", "run_command"]
required_commands = ["python3"]
required_variables = ["PROJECT_MODE"]
requires_network = false
```

O manifesto não concede permissões nem injeta variáveis no container. Ele exige compatibilidade com o perfil e ambiente aprovados. `source` não aceita credenciais/query; mudanças do manifesto entram no snapshot de aprovação. Skills sem manifesto são identificadas como locais, com hash de conteúdo e licença não especificada; o Kraken não inventa proveniência ou licença. Adaptar instruções e avaliar adequação semântica continua necessário, mesmo que requisitos declarados sejam satisfeitos.

## Resultados de verificações

Checks podem definir `runner` (`auto`, `command`, `unittest`, `pytest`, `junit`), `min_passed` (mínimo 1), `allow_skipped` e `allow_expected_failures` (ambos falsos por padrão). Um check funcional como `cmp` pode passar sem contagem de testes; uma suíte identificada não pode passar apenas por retornar exit 0.

Para JUnit, declare `report = "/tmp/result.xml"` e faça o comando escrever nesse arquivo. O executor monta um arquivo de saída exclusivo, preservando a raiz do projeto somente para leitura. Relatórios ausentes, vazios, inconsistentes ou de outra execução não aprovam a tarefa. O produtor deve escrever diretamente no arquivo declarado; uma substituição atômica por rename de mountpoint não é suportada. XML com entidades/DOCTYPE é recusado.

Skips, xfails, sucessos inesperados e testes desabilitados são registrados separadamente. Mesmo permitindo skips/xfails, zero resultados aprovados nunca satisfaz `min_passed`. Saída de runner truncada exige relatório verificável, em vez de inferir sucesso do texto parcial. Os mesmos controles valem para sensores de saúde.
