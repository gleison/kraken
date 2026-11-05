"""Validated, serializable contracts; no model or UI dependencies."""
from dataclasses import asdict, dataclass, field
from hashlib import sha256
import json
from pathlib import PurePosixPath
import re


PLAN_REVIEW_CRITERIA = ('scope', 'decisions', 'dependencies', 'acceptance', 'harness',
                        'environment', 'limits', 'team', 'self_containment')


class KrakenError(Exception):
    """An actionable failure safe to present after sanitization."""


class DecisionRequired(KrakenError):
    """The approved contract cannot resolve this decision autonomously."""


def canonical(value) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':'), allow_nan=False)


def digest(value) -> str:
    return sha256(canonical(value).encode()).hexdigest()


def require(condition, message):
    if not condition:
        raise KrakenError(message)


def safe_path(value: str) -> str:
    require(isinstance(value, str) and bool(value), 'Caminho vazio ou inválido')
    path = PurePosixPath(value)
    require(not path.is_absolute() and '..' not in path.parts and '\\' not in value and not any(c in value for c in (',', '\x00', '\n', '\r')),
            f'Caminho fora do projeto: {value}')
    require(path.parts and path.parts[0] not in {'.git', '.kraken'}, 'Metadados protegidos')
    return path.as_posix()


def identifier(value: str) -> str:
    require(isinstance(value, str) and re.fullmatch(r'[A-Za-z][A-Za-z0-9_-]{0,63}', value),
            'Identificador inválido')
    return value


@dataclass(frozen=True)
class Limits:
    concurrency: int = 2
    calls: int = 100
    active_seconds: int = 1800
    command_seconds: int = 120
    attempt_seconds: int = 600
    attempts: int = 3
    turns: int = 20
    output_tokens: int = 4096
    # Mandatory acknowledgement when a monetary ceiling cannot be measured.
    counter_only: bool = False
    max_estimated_cost_usd: float | None = None

    def __post_init__(self):
        for key, value in asdict(self).items():
            if key not in {'counter_only', 'max_estimated_cost_usd'}:
                require(type(value) is int and 0 < value <= 1_000_000, f'Limite inválido: {key}')
        require(self.max_estimated_cost_usd is None or (type(self.max_estimated_cost_usd) in (int, float)
                and 0 < self.max_estimated_cost_usd < 1_000_000), 'Limite de custo estimado inválido')
        require(self.concurrency <= 16, 'Concorrência máxima: 16')
        require(type(self.counter_only) is bool, 'counter_only precisa ser booleano')


@dataclass(frozen=True)
class Criterion:
    id: str
    description: str

    def __post_init__(self):
        identifier(self.id)
        require(bool(self.description.strip()), 'Critério sem descrição')


@dataclass(frozen=True)
class Check:
    id: str
    argv: tuple[str, ...]
    criteria: tuple[str, ...]
    category: str = 'behaviour'
    timing: str = 'before_integration'

    def __post_init__(self):
        identifier(self.id)
        require(bool(self.argv) and all(isinstance(x, str) and '\x00' not in x for x in self.argv),
                'Verificação precisa de argv válido')
        require(self.category in {'behaviour', 'maintainability', 'architecture'}, 'Categoria inválida')
        require(self.timing in {'before_integration', 'after_integration', 'health'}, 'Momento inválido')
        require(bool(self.criteria), 'Verificação sem critérios')


@dataclass(frozen=True)
class Task:
    id: str
    title: str
    goal: str
    role: str
    allowed_paths: tuple[str, ...]
    criteria: tuple[Criterion, ...]
    checks: tuple[Check, ...]
    depends_on: tuple[str, ...] = ()
    references: tuple[str, ...] = ()

    def __post_init__(self):
        identifier(self.id)
        identifier(self.role)
        require(bool(self.title.strip()) and bool(self.goal.strip()), 'Tarefa sem título/objetivo')
        require(bool(self.allowed_paths), 'Defina caminhos permitidos explicitamente')
        for path in self.allowed_paths:
            safe_path(path)
            require(not any(x in path for x in '*?['), 'Use arquivos ou diretórios, sem globs')
        ids = {c.id for c in self.criteria}
        require(bool(ids) and len(ids) == len(self.criteria), 'Critérios vazios ou duplicados')
        require(len({c.id for c in self.checks}) == len(self.checks), 'Checks duplicados')
        require(any(c.category == 'behaviour' and c.timing == 'before_integration' for c in self.checks),
                'Tarefa exige ao menos uma verificação comportamental antes da integração')
        covered = set()
        for check in self.checks:
            require(set(check.criteria) <= ids, 'Check referencia critério ausente')
            if check.timing != 'health':
                covered.update(check.criteria)
        require(covered == ids, 'Todo critério exige verificação executável')
        for path in self.references:
            safe_path(path)

    def permits(self, path: str) -> bool:
        path = safe_path(path)
        return any(path == p or path.startswith(p.rstrip('/') + '/') for p in self.allowed_paths)


@dataclass(frozen=True)
class Plan:
    objective: str
    scope: str
    exclusions: str
    decisions: tuple[str, ...]
    tasks: tuple[Task, ...]
    image: str
    limits: Limits = field(default_factory=Limits)
    glossary: dict[str, str] = field(default_factory=dict)
    uncertainties: tuple[str, ...] = ()
    # Immutable, user-reviewed examples/fixtures; never editable by agents.
    fixtures: dict[str, str] = field(default_factory=dict)

    def __post_init__(self):
        require(all(isinstance(s, str) and s.strip() for s in
                    (self.objective, self.scope, self.exclusions)), 'Plano incompleto')
        require(bool(self.decisions) and all(isinstance(s, str) and s.strip() for s in self.decisions),
                'Decisões obrigatórias')
        require(re.fullmatch(r'[A-Za-z0-9._/:\-]+@sha256:[0-9a-f]{64}', self.image),
                'Imagem precisa estar fixada por digest sha256')
        require(bool(self.tasks), 'Plano sem tarefas')
        tasks = {task.id: task for task in self.tasks}
        require(len(tasks) == len(self.tasks), 'Tarefas duplicadas')
        for path, expected in self.fixtures.items():
            safe_path(path)
            require(bool(re.fullmatch('[0-9a-f]{64}', expected)), 'Hash de fixture inválido')
            require(not any(task.permits(path) for task in self.tasks), 'Fixture aprovada é protegida')
        active, visited = set(), set()

        def visit(key):
            require(key in tasks, f'Dependência ausente: {key}')
            require(key not in active, 'Ciclo de dependências')
            if key in visited:
                return
            active.add(key)
            for parent in tasks[key].depends_on:
                visit(parent)
            active.remove(key)
            visited.add(key)
        for key in tasks:
            visit(key)

    @property
    def revision(self):
        return digest(self.to_dict())

    def to_dict(self):
        return asdict(self)

    @classmethod
    def from_dict(cls, data):
        try:
            data = dict(data)
            data['limits'] = Limits(**data.get('limits', {}))
            tasks = []
            for raw in data['tasks']:
                raw = dict(raw)
                raw['criteria'] = tuple(Criterion(**c) for c in raw['criteria'])
                raw['checks'] = tuple(Check(**{**c, 'argv': tuple(c['argv']),
                                                    'criteria': tuple(c['criteria'])}) for c in raw['checks'])
                for key in ('allowed_paths', 'depends_on', 'references'):
                    raw[key] = tuple(raw.get(key, ()))
                tasks.append(Task(**raw))
            data['tasks'] = tuple(tasks)
            data['decisions'] = tuple(data.get('decisions', ()))
            data['uncertainties'] = tuple(data.get('uncertainties', ()))
            return cls(**data)
        except (KeyError, TypeError, AttributeError, ValueError) as exc:
            raise KrakenError('Estrutura de plano inválida; confira o contrato documentado') from exc

    def render(self):
        lines = [f'# Plano — {self.revision[:12]}', '', self.objective,
                 '', '## Escopo', self.scope, '', '## Exclusões', self.exclusions,
                 '', '## Decisões', *[f'- {d}' for d in self.decisions],
                 '', '## Limites', '```json', json.dumps(asdict(self.limits), indent=2), '```',
                 '', 'Custo monetário: indisponível. Aprovação por contadores: ' +
                 ('sim' if self.limits.counter_only else 'NÃO'), '', '## Ambiente',
                 self.image, 'Rede dos executores: desabilitada.', '', '## Incertezas',
                 *[f'- {u}' for u in self.uncertainties], '', '## Tarefas']
        for task in self.tasks:
            lines += [f'### {task.id} — {task.title}', task.goal, f'Papel: {task.role}',
                      'Dependências: ' + ', '.join(task.depends_on),
                      'Caminhos: ' + ', '.join(task.allowed_paths)]
            lines += [f'- {c.id}: {c.description}' for c in task.criteria]
            for check in task.checks:
                lines.append(f'- Check {check.id}: {list(check.argv)!r}; {check.category}; {check.timing}')
        lines += ['', '## Contrato integral', '```json', json.dumps(self.to_dict(), ensure_ascii=False, indent=2), '```']
        return '\n'.join(lines) + '\n'
