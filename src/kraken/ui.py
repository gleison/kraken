"""Textual adapter; all project state belongs to Session/core, not widgets."""
import asyncio
import json

from textual.app import App, ComposeResult
from textual.containers import Horizontal, VerticalScroll
from textual.screen import ModalScreen
from textual.widgets import Button, DataTable, Footer, Header, Input, Markdown, RichLog, Static, TabbedContent, TabPane

from .config import Config
from .domain import KrakenError, canonical


class ApprovePlan(ModalScreen):
    CSS = 'ApprovePlan { align: center middle; } #dialog { width: 95%; height: 95%; border: solid cyan; background: $surface; }'

    def __init__(self, plan, config):
        super().__init__()
        self.plan, self.config = plan, config

    def compose(self):
        with VerticalScroll(id='dialog'):
            yield Markdown(self.plan.render() + '\n## Modelos, papéis e skills aprovados\n```json\n' +
                           json.dumps(self.config.snapshot(), ensure_ascii=False, indent=2) + '\n```')
            yield Static('Confirmar autoriza implementar ESTA revisão, dentro dos limites exibidos. '
                         'Não autoriza push, merge na principal ou deploy. Custo monetário não é garantido.')
            with Horizontal():
                yield Button('Confirmar aprovação', id='confirm', variant='success')
                yield Button('Voltar sem aprovar', id='dismiss')

    def on_button_pressed(self, event: Button.Pressed):
        if event.button.id == 'confirm':
            self.dismiss((self.plan.revision, self.config.revision))
        elif event.button.id == 'dismiss':
            self.dismiss(None)


class TaskEvidence(ModalScreen):
    CSS = 'TaskEvidence { background: $surface; } #evidence-list { height: 30%; } #evidence-records { height: 1fr; }'

    def __init__(self, task, records, read):
        super().__init__()
        self.task_id, self.records, self.read = task, records, read

    def compose(self):
        yield Static(f'Evidências de {self.task_id} — até 100 registros, mais recentes primeiro')
        yield DataTable(id='evidence-list', cursor_type='row')
        yield RichLog(id='evidence-records', wrap=True, markup=False, max_lines=10000)
        yield Button('Voltar', id='evidence-close')

    def on_mount(self):
        table = self.query_one('#evidence-list', DataTable)
        table.add_columns('ID', 'Tentativa', 'Tipo', 'Resultado', 'Versão')
        for record in self.records:
            table.add_row(record['id'], str(record['attempt']), record['kind'],
                          'aprovada' if record['passed'] else 'reprovada', record['version'], key=record['id'])
        if not self.records:
            self.query_one('#evidence-records', RichLog).write('Nenhuma evidência persistida para esta tarefa.')

    def on_data_table_row_highlighted(self, event: DataTable.RowHighlighted):
        log = self.query_one('#evidence-records', RichLog)
        log.clear()
        text = json.dumps(self.read(event.row_key.value), ensure_ascii=False, indent=2)
        log.write(text[:60000])
        if len(text) > 60000:
            log.write('Visualização limitada a 60 mil caracteres. O registro integral permanece no estado da sessão.')

    def on_button_pressed(self, event: Button.Pressed):
        event.stop()
        if event.button.id == 'evidence-close':
            self.dismiss(None)


class KrakenApp(App):
    TITLE = 'Kraken — Orquestrador'
    BINDINGS = [('q', 'request_quit', 'Sair'), ('p', 'pause_run', 'Pausar'), ('c', 'cancel_run', 'Cancelar')]
    CSS = '''
    #summary { height: auto; max-height: 5; padding: 0 1; }
    #tasks { height: 1fr; }
    #actions { height: 3; overflow-x: auto; }
    #actions Button { min-width: 12; width: auto; }
    #message { dock: bottom; }
    #send { dock: bottom; }
    #activity { height: 1fr; }
    '''

    def __init__(self, session, *, catalog=None):
        super().__init__()
        self.session = session
        self.catalog = catalog
        self.last_event = 0
        self.busy = False
        self.background = None
        self._known_tasks = set()
        self._rendered_revision = None
        self._closing = False

    def compose(self) -> ComposeResult:
        yield Header()
        yield Static('Sem chamadas automáticas. Enviar/revisar usa API e pode gerar custo.', id='summary')
        with Horizontal(id='actions'):
            yield Button('Projetos/sessões', id='manage', disabled=self.catalog is None)
            yield Button('Revisar/corrigir (API)', id='review')
            yield Button('Aprovar plano', id='approve')
            yield Button('Executar/retomar', id='run', variant='primary')
            yield Button('Pausar', id='pause')
            yield Button('Cancelar', id='cancel', variant='error')
            yield Button('Reconciliar (API se necessário)', id='retry')
        with TabbedContent():
            with TabPane('Planejamento', id='planning-tab'):
                yield RichLog(id='conversation', wrap=True, markup=False, max_lines=1000)
                yield Button('Enviar à API (pode gerar custo)', id='send')
                yield Input(placeholder='Intenção, resposta ou alteração do plano…', id='message')
            with TabPane('Plano integral', id='plan-tab'):
                with VerticalScroll():
                    yield Markdown('Nenhum plano gerado ou importado.', id='plan')
            with TabPane('Tarefas', id='tasks-tab'):
                yield Button('Inspecionar evidências da tarefa', id='inspect-evidence')
                yield DataTable(id='tasks', cursor_type='row')
            with TabPane('Agentes', id='agents-tab'):
                yield DataTable(id='agents', cursor_type='row')
            with TabPane('Atividade e evidências', id='activity-tab'):
                yield RichLog(id='activity', wrap=True, markup=False, max_lines=2000)
        yield Footer()

    def on_mount(self):
        self._agents = self.query_one('#agents', DataTable)
        self._agents.add_columns('Atribuição', 'Papel', 'Modelo', 'Estado')
        self._known_agents = set()
        self.query_one('#tasks', DataTable).add_columns('Tarefa', 'Dependências', 'Estado', 'Tentativa', 'Detalhe')
        for entry in self.session.store.get('interview', [])[-20:]:
            self.query_one('#conversation', RichLog).write('Você: ' + entry['user'])
            self.query_one('#conversation', RichLog).write('Planejamento: ' + entry['response'])
        self._summary = self.query_one('#summary', Static)
        self._table = self.query_one('#tasks', DataTable)
        self._activity = self.query_one('#activity', RichLog)
        self._plan_md = self.query_one('#plan', Markdown)
        self._timer = self.set_interval(0.25, self.refresh_state)
        self.refresh_state()

    def refresh_state(self):
        if self._closing:
            return
        store = self.session.store
        summary = (f'Sessão {store.get("session")} | {store.get("control")} | '
                   f'Chamadas: {store.get("calls", 0)} | Uso conhecido: {store.get("usage", {})} | '
                   f'Custo estimado USD: {store.get("estimated_cost_usd") if not store.get("cost_unavailable", True) else "indisponível"} | API por ação explícita')
        self._summary.update(summary)
        table = self._table
        try:
            plan = self.session.plan
            dependencies = {t.id: ', '.join(t.depends_on) or '—' for t in plan.tasks} if plan else {}
        except KrakenError:
            dependencies = {}
        for row in store.tasks():
            values = (row['id'], dependencies.get(row['id'], '—'), row['status'], str(row['attempt']), row['detail'][:180])
            if row['id'] not in self._known_tasks:
                table.add_row(*values, key=row['id'])
                self._known_tasks.add(row['id'])
            else:
                for column, value in zip(table.columns, values):
                    table.update_cell(row['id'], column, value)
        live = {r['id'] for r in store.tasks()}
        for removed in self._known_tasks - live:
            table.remove_row(removed)
        self._known_tasks = live
        for event in store.events(self.last_event):
            self.last_event = event['seq']
            self._activity.write(event['kind'] + ' ' + canonical(event['payload']))
            data = event['payload']
            if event['kind'] == 'agent_started' and data['assignment'] not in self._known_agents:
                self._agents.add_row(data['assignment'], data['role'], data['model'], 'ativo', key=data['assignment'])
                self._known_agents.add(data['assignment'])
            elif event['kind'] == 'agent_finished' and data['assignment'] in self._known_agents:
                self._agents.update_cell(data['assignment'], list(self._agents.columns)[3], data['status'])
            elif event['kind'] == 'recovery_required':
                for key in self._known_agents:
                    self._agents.update_cell(key, list(self._agents.columns)[3], 'conferir histórico')
        try:
            plan = self.session.plan
            revision = plan.revision if plan else None
            if revision != self._rendered_revision:
                self._plan_md.update(plan.render() if plan else 'Nenhum plano.')
                self._rendered_revision = revision
        except KrakenError:
            pass  # Explicit actions report errors; avoid repeatedly flooding the log on refresh.

    def launch(self, operation):
        if self.busy:
            self.query_one('#activity', RichLog).write('Aguarde a operação ativa; pausa/cancelamento continuam disponíveis.')
            operation.close()
            return
        self.busy = True

        async def run():
            try:
                result = await operation
                if result:
                    self.query_one('#conversation', RichLog).write(self.session.store.redactor.text(result))
            except (KrakenError, OSError, ValueError) as exc:
                message = self.session.store.redactor.text(exc)
                self.session.store.emit('operation_failed', message=message)
                self.query_one('#activity', RichLog).write(message)
                self.notify(message, severity='error', timeout=8)
            finally:
                self.busy = False
                self.refresh_state()
        self.background = asyncio.create_task(run())

    def on_button_pressed(self, event: Button.Pressed):
        action = event.button.id
        if action == 'manage':
            if self.busy:
                self.notify('Pause/encerre a operação antes de trocar de sessão.')
            elif self.catalog is not None:
                self.push_screen(ProjectManager(self.catalog, self.session.root), self.switch_project)
        elif action == 'send':
            field = self.query_one('#message', Input)
            text = field.value
            if not text.strip() or self.busy:
                return
            field.value = ''
            self.query_one('#conversation', RichLog).write('Você: ' + self.session.store.redactor.text(text))
            self.launch(self.session.ask(text))
        elif action == 'review':
            self.launch(self.session.review())
        elif action == 'run':
            self.launch(self.session.run())
        elif action == 'pause':
            self.action_pause_run()
        elif action == 'cancel':
            self.action_cancel_run()
        elif action == 'approve':
            if self.busy:
                return
            try:
                plan = self.session.plan
                if plan is None:
                    raise KrakenError('Gere ou importe um plano primeiro')
                self.push_screen(ApprovePlan(plan, Config(self.session.root)), self.approved)
            except KrakenError as exc:
                self.notify(str(exc), severity='error')
        elif action == 'retry':
            table = self.query_one('#tasks', DataTable)
            if table.row_count:
                task_id = table.get_row_at(table.cursor_row)[0]
                self.launch(self.session.retry(task_id))
        elif action == 'inspect-evidence':
            table = self.query_one('#tasks', DataTable)
            if table.row_count:
                task_id = table.get_row_at(table.cursor_row)[0]
                self.push_screen(TaskEvidence(task_id, self.session.store.task_evidence(task_id, summary=True),
                                              self.session.store.evidence_record))

    def switch_project(self, choice):
        if choice:
            self.exit(choice)

    def approved(self, decision):
        if decision:
            try:
                self.session.approve(*decision)
                self.notify('Revisão aprovada explicitamente. Executar é uma ação separada.')
            except KrakenError as exc:
                self.notify(str(exc), severity='error', timeout=8)
        self.refresh_state()

    def action_pause_run(self):
        self.session.pause()

    def action_cancel_run(self):
        self.session.cancel()
        if self.busy and self.session.orchestrator is None and self.background:
            self.background.cancel()

    def action_request_quit(self):
        if self.busy:
            self.action_cancel_run()
            self.notify('Cancelamento solicitado. Aguarde a confirmação e pressione q novamente.')
        else:
            self.exit()

    async def on_unmount(self):
        self._closing = True
        if hasattr(self, '_timer'):
            self._timer.stop()
        if self.background and not self.background.done():
            self.session.cancel()
            self.background.cancel()
            await asyncio.gather(self.background, return_exceptions=True)


class ProjectManager(ModalScreen):
    """Selection only. No model, runtime or project task starts from this screen."""
    CSS = '''
    ProjectManager { background: $surface; }
    #project-list { height: 30%; }
    #session-list { height: 35%; }
    #manager-actions { height: 3; }
    '''

    def __init__(self, catalog, current=None):
        super().__init__()
        self.catalog, self.current = catalog, current

    def compose(self):
        yield Static('Projetos e sessões — selecionar não executa tarefas nem chama APIs')
        yield Input(value=str(self.current or ''), placeholder='Raiz de um projeto Git', id='project-path')
        yield DataTable(id='project-list', cursor_type='row')
        yield Static('Sessões preservadas do projeto selecionado')
        yield DataTable(id='session-list', cursor_type='row')
        with Horizontal(id='manager-actions'):
            yield Button('Abrir projeto', id='manager-open')
            yield Button('Retomar sessão', id='manager-resume')
            yield Button('Nova sessão', id='manager-new')
            yield Button('Voltar', id='manager-back')

    def on_mount(self):
        self.query_one('#project-list', DataTable).add_columns('Projeto', 'Caminho')
        self.query_one('#session-list', DataTable).add_columns('ID', 'Estado', 'Chamadas', 'Tarefas', 'Objetivo')
        for index, path in enumerate(self.catalog.projects()):
            self.query_one('#project-list', DataTable).add_row(path.name, str(path), key=str(index))
        if self.current:
            self.show_sessions(self.current)

    def show_sessions(self, root):
        from .projects import sessions
        table = self.query_one('#session-list', DataTable)
        table.clear()
        for row in sessions(root):
            table.add_row(row['id'], row['status'] or 'idle', str(row['calls']),
                          canonical(row['tasks']), row['objective'], key=row['id'])

    def on_data_table_row_highlighted(self, event: DataTable.RowHighlighted):
        if event.data_table.id == 'project-list':
            root = event.data_table.get_row(event.row_key)[1]
            self.query_one('#project-path', Input).value = root
            self.show_sessions(root)

    async def on_button_pressed(self, event: Button.Pressed):
        event.stop()
        action = event.button.id
        if action == 'manager-back':
            self.dismiss(None)
            return
        if action not in {'manager-open', 'manager-new', 'manager-resume'}:
            return
        from pathlib import Path
        from .projects import init_project
        root = Path(self.query_one('#project-path', Input).value).expanduser().resolve()
        try:
            await init_project(root)
            self.catalog.add(root)
            selected = None
            if action == 'manager-new':
                selected = 'new'
            elif action == 'manager-resume':
                table = self.query_one('#session-list', DataTable)
                if not table.row_count:
                    raise KrakenError('Selecione um projeto com sessão existente')
                selected = table.get_row_at(table.cursor_row)[0]
            self.dismiss((str(root), selected))
        except (KrakenError, OSError) as exc:
            self.notify(str(exc), severity='error')


class ProjectLauncher(App):
    TITLE = 'Kraken — Projetos e sessões'
    BINDINGS = [('q', 'quit', 'Sair')]

    def __init__(self, catalog):
        super().__init__()
        self.catalog = catalog

    def on_mount(self):
        self.push_screen(ProjectManager(self.catalog), self.exit)
