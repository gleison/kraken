"""Configuration contains credential variable names, never credential values."""
from dataclasses import asdict, dataclass
from hashlib import sha256
from importlib.resources import files
import json
import os
from pathlib import Path
import re
import tomllib
from urllib.parse import urlsplit

from .domain import KrakenError, digest, identifier, require

TOOLS = {'read_file', 'list_files', 'run_command', 'write_file'}
READ_TOOLS = ('read_file', 'list_files')
ROLES = {'orchestrator': ('coordination', READ_TOOLS),
         'product': ('planning', READ_TOOLS),
         'backend': ('implementation', tuple(sorted(TOOLS))),
         'frontend': ('implementation', tuple(sorted(TOOLS))),
         'platform': ('harness', tuple(sorted(TOOLS))),
         'qa': ('qa', READ_TOOLS + ('run_command',)),
         'reviewer': ('review', READ_TOOLS + ('run_command',)),
         'security': ('review', READ_TOOLS + ('run_command',)),
         'ux': ('review', READ_TOOLS)}


@dataclass(frozen=True)
class Profile:
    base_url: str
    model: str
    api_key_env: str = 'OPENAI_API_KEY'
    timeout: int = 60
    input_usd_per_million: float | None = None
    output_usd_per_million: float | None = None

    def __post_init__(self):
        parsed = urlsplit(self.base_url)
        local = parsed.hostname in {'127.0.0.1', 'localhost', '::1'}
        require(parsed.scheme == 'https' or (parsed.scheme == 'http' and local),
                'Endpoint exige HTTPS; HTTP permitido apenas em loopback')
        require(parsed.hostname and not parsed.username and not parsed.password and
                not parsed.query and not parsed.fragment, 'Endpoint inválido ou contém credencial')
        require(isinstance(self.model, str) and bool(self.model.strip()), 'Defina OPENAI_MODEL ou model no perfil')
        require(not self.api_key_env or re.fullmatch('[A-Za-z_][A-Za-z0-9_]*', self.api_key_env),
                'Nome de variável de credencial inválido')
        for price in (self.input_usd_per_million, self.output_usd_per_million):
            require(price is None or (type(price) in (int, float) and 0 <= price < 1_000_000),
                    'Preço inválido')
        require(type(self.timeout) is int and self.timeout > 0, 'Timeout inválido')

    def key(self):
        if not self.api_key_env:
            return ''
        value = os.environ.get(self.api_key_env, '')
        require(bool(value), f'Credencial ausente: configure {self.api_key_env} no ambiente')
        return value


@dataclass(frozen=True)
class Role:
    name: str
    profile: Profile
    tools: tuple[str, ...]
    skill_name: str
    skill: str
    skill_hash: str

    def snapshot(self):
        return {'name': self.name, 'profile': asdict(self.profile) if self.profile else None, 'tools': self.tools,
                'skill': self.skill_name, 'skill_hash': self.skill_hash}


class Config:
    def __init__(self, root: Path):
        try:
            self._load(root)
        except (TypeError, ValueError, AttributeError, KeyError, OSError) as exc:
            raise KrakenError('Configuração inválida; confira tabelas, tipos e arquivos de skills') from exc

    def _load(self, root: Path):
        self.root = root.resolve()
        path = self.root / '.kraken/project.toml'
        try:
            self.raw = tomllib.loads(path.read_text()) if path.exists() else {}
        except (tomllib.TOMLDecodeError, OSError) as exc:
            raise KrakenError('Configuração TOML inválida') from exc
        self.docker_context = self.raw.get('execution', {}).get('docker_context', 'default')
        require(re.fullmatch(r'[A-Za-z0-9_.-]+', self.docker_context), 'Contexto Docker inválido')
        self._roles = {}
        names = set(ROLES) | set(self.raw.get('roles', {}))
        for name in sorted(names):
            identifier(name)
            data = self.raw.get('roles', {}).get(name, {})
            default = self.raw.get('provider', {})
            env = {'base_url': os.environ.get('OPENAI_BASE_URL'), 'model': os.environ.get('OPENAI_MODEL')}
            defaults = {'base_url': 'https://api.openai.com/v1', 'model': '',
                        'api_key_env': 'OPENAI_API_KEY', 'timeout': 60}
            profile_data = defaults | default | {k: v for k, v in env.items() if v is not None}
            profile_data.update(data.get('provider', {}))
            # Config can be inspected and initialized before a model is selected.
            profile = None if not profile_data['model'] else Profile(**profile_data)
            skill_name, tools = ROLES.get(name, ('implementation', READ_TOOLS))
            tools = tuple(data.get('tools', tools))
            require(set(tools) <= TOOLS, f'Ferramenta desconhecida no papel {name}')
            skill_name = data.get('skill', skill_name)
            identifier(skill_name)
            local = self.root / '.kraken/skills' / skill_name / 'SKILL.md'
            if local.exists():
                require(local.resolve().is_relative_to(self.root / '.kraken/skills'),
                        'Skill fora do catálogo local')
                content = local.read_text()
            else:
                bundled = files('kraken').joinpath('skills', skill_name, 'SKILL.md')
                require(bundled.is_file(), f'Skill não encontrada: {skill_name}')
                content = bundled.read_text()
            require(len(content) <= 32_000, 'Skill excede limite de contexto')
            require(not re.search(r'\b(?:claude|codex)\s+(?:--|plugin|exec)|Call the Skill tool', content),
                    f'Skill {skill_name} depende de um runtime externo; adapte-a antes de usar')
            self._roles[name] = Role(name, profile, tools, skill_name, content,
                                     sha256(content.encode()).hexdigest())

    def role(self, name):
        require(name in self._roles, f'Papel não configurado: {name}')
        role = self._roles[name]
        require(role.profile is not None, f'Defina um modelo para {name}')
        return role

    def snapshot(self):
        return {'docker_context': self.docker_context, 'roles': {name: role.snapshot() for name, role in self._roles.items()}}

    @property
    def revision(self):
        return digest(self.snapshot())

    def secrets(self):
        return [os.environ.get(role.profile.api_key_env, '') for role in self._roles.values()
                if role.profile and role.profile.api_key_env]


class Redactor:
    def __init__(self, values=()):
        self.values = sorted({v for v in values if v}, key=len, reverse=True)

    def text(self, text):
        text = str(text)
        for value in self.values:
            text = text.replace(value, '[REDACTED]')
        text = re.sub(r'(?i)(bearer\s+)[^\s"\']+', r'\1[REDACTED]', text)
        text = re.sub(r'(?i)((?:api[_-]?key|token|password|secret)\s*[=:]\s*)[^\s,;]+',
                      r'\1[REDACTED]', text)
        return text

    def data(self, data):
        # Redact before serialization to keep the JSON structurally valid.
        if isinstance(data, str):
            return self.text(data)
        if isinstance(data, dict):
            return {k: self.data(v) for k, v in data.items()}
        if isinstance(data, (list, tuple)):
            return [self.data(v) for v in data]
        return data
