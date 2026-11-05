"""Portable, explicit skill requirements; no YAML parser or executable hooks."""
from hashlib import sha256
from pathlib import Path
import re
import tomllib
from urllib.parse import urlsplit

from .domain import KrakenError, require


def manifest(resource, content, origin):
    exists = resource.is_file()
    if isinstance(resource, Path) and exists:
        require(not resource.is_symlink(), 'Manifesto de skill não pode ser symlink')
    try:
        raw = tomllib.loads(resource.read_text()) if exists else {}
    except (OSError, tomllib.TOMLDecodeError) as exc:
        raise KrakenError('Manifesto skill.toml inválido') from exc
    fields = {'source', 'version', 'license', 'runtime', 'required_tools', 'required_commands',
              'required_variables', 'requires_network'}
    require(set(raw) <= fields, 'Requisito de skill desconhecido; não será ignorado')
    result = {'source': origin, 'version': 'sha256:' + sha256(content.encode()).hexdigest(),
              'license': 'unspecified', 'runtime': 'kraken', 'required_tools': [],
              'required_commands': [], 'required_variables': [], 'requires_network': False,
              **raw, 'declared': exists}
    for key in ('source', 'version', 'license', 'runtime'):
        require(isinstance(result[key], str) and result[key].strip(), 'Metadado de skill vazio/inválido')
    parsed = urlsplit(result['source'])
    require(not parsed.username and not parsed.password and not parsed.query, 'Origem da skill contém credenciais/query')
    require(result['runtime'] == 'kraken', 'Skill exige runtime não suportado')
    require(type(result['requires_network']) is bool and not result['requires_network'],
            'Skill exige rede; executores deste MVP são offline')
    for key in ('required_tools', 'required_commands', 'required_variables'):
        require(isinstance(result[key], list) and all(isinstance(v, str) for v in result[key]),
                'Requisitos de skill devem ser listas de nomes')
    for name in result['required_commands']:
        require(re.fullmatch(r'[A-Za-z0-9_.+-]+', name), 'Nome de executável inválido')
    for name in result['required_variables']:
        require(re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*', name), 'Nome de variável inválido')
    return result
