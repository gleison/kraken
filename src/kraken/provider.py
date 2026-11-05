"""Small OpenAI-compatible Chat Completions adapter over HTTPX."""
import asyncio
from dataclasses import dataclass
import json
import ssl
import time

import httpx

from .domain import KrakenError, Limits, canonical, require


@dataclass
class Reply:
    content: str
    tool_calls: list[dict]
    usage: dict | None

    def message(self):
        result = {'role': 'assistant', 'content': self.content or None}
        if self.tool_calls:
            result['tool_calls'] = self.tool_calls
        return result


class Budget:
    def __init__(self, store, limits=None):
        self.store = store
        self.limits = limits or Limits()

    def check(self):
        require(self.store.get('calls', 0) < self.limits.calls, 'Limite de chamadas atingido')
        require(self.store.get('active_seconds', 0) < self.limits.active_seconds,
                'Limite de tempo ativo atingido')
        ceiling = self.limits.max_estimated_cost_usd
        require(ceiling is None or self.store.get('estimated_cost_usd', 0.0) < ceiling,
                'Limite de custo estimado atingido')
        require(self.store.get('control') != 'cancelling', 'Cancelamento solicitado')

    def reserve(self, estimate=0.0):
        self.check()
        ceiling = self.limits.max_estimated_cost_usd
        projected = (self.store.get('estimated_cost_usd', 0.0)
                     + self.store.get('reserved_cost_usd', 0.0) + estimate)
        require(ceiling is None or projected <= ceiling, 'Limite de custo estimado atingido')
        self.store.set('reserved_cost_usd', self.store.get('reserved_cost_usd', 0.0) + estimate)
        # This method contains no await: reservation is atomic on the owner event loop.
        self.store.set('calls', self.store.get('calls', 0) + 1)
        return estimate

    def settle(self, reserved, cost):
        self.store.set('reserved_cost_usd', max(0.0, self.store.get('reserved_cost_usd', 0.0) - reserved))
        self.store.set('estimated_cost_usd', self.store.get('estimated_cost_usd', 0.0) + cost)

    def elapsed(self, seconds):
        self.store.set('active_seconds', self.store.get('active_seconds', 0) + seconds)


class ModelClient:
    def __init__(self, store, budget, client=None):
        self.store, self.budget = store, budget
        self.client = client

    async def complete(self, role, messages, tools=(), tool_choice=None):
        profile = role.profile
        key = profile.key()
        encoded = canonical(messages)
        require(len(encoded) <= 100_000, 'Contexto excede o limite de 100 mil caracteres; reduza-o')
        priced = profile.input_usd_per_million is not None and profile.output_usd_per_million is not None
        require(self.budget.limits.max_estimated_cost_usd is None or priced,
                'Limite estimado exige preços de entrada e saída em todos os perfis usados')
        # Bytes are a conservative planning proxy, not a universal tokenizer guarantee.
        estimate = (((len(encoded.encode()) + len(canonical(list(tools)).encode())) * profile.input_usd_per_million +
                    self.budget.limits.output_tokens * profile.output_usd_per_million) / 1_000_000
                    if priced else 0.0)
        reserved = self.budget.reserve(estimate)
        cost = estimate
        started = time.monotonic()
        self.store.emit('model_started', role=role.name, model=profile.model,
                        call=self.store.get('calls'))
        body = {'model': profile.model, 'messages': messages, 'stream': True,
                'stream_options': {'include_usage': True},
                'max_tokens': self.budget.limits.output_tokens}
        require(len(canonical(messages)) <= 100_000, 'Contexto excede o limite de 100 mil caracteres; reduza-o')
        if tools:
            body['tools'] = list(tools)
        if tool_choice is not None:
            body['tool_choice'] = tool_choice
        headers = {'Authorization': f'Bearer {key}'} if key else {}
        client = self.client or httpx.AsyncClient(verify=ssl.create_default_context(),
                                                 timeout=profile.timeout)
        try:
            remaining = self.budget.limits.active_seconds - self.store.get('active_seconds', 0)
            async with asyncio.timeout(min(profile.timeout, remaining)):
                async with client.stream('POST', profile.base_url.rstrip('/') + '/chat/completions',
                                         json=body, headers=headers) as response:
                    response.raise_for_status()
                    reply = await self._read(response)
            if reply.usage is not None:
                if priced:
                    cost = (reply.usage['prompt_tokens'] * profile.input_usd_per_million +
                            reply.usage['completion_tokens'] * profile.output_usd_per_million) / 1_000_000
                usage = self.store.get('usage', {'prompt_tokens': 0, 'completion_tokens': 0})
                for field in usage:
                    usage[field] += reply.usage.get(field, 0)
                self.store.set('usage', usage)
            else:
                self.store.set('unknown_usage_calls', self.store.get('unknown_usage_calls', 0) + 1)
            self.store.emit('model_finished', role=role.name, usage=reply.usage,
                            estimated_cost_usd=cost if priced else None, tools=[c['function']['name'] for c in reply.tool_calls])
            return reply
        except httpx.HTTPStatusError as exc:
            raise KrakenError(f'API retornou HTTP {exc.response.status_code}; confira credencial, endpoint e limites') from exc
        except (httpx.HTTPError, TimeoutError) as exc:
            self.store.emit('model_uncertain', role=role.name,
                            reason='Falha de transporte; o provedor pode ter consumido recursos')
            raise KrakenError('Falha/timeout na API; não foi feita repetição automática') from exc
        except asyncio.CancelledError:
            self.store.emit('model_uncertain', role=role.name, reason='Chamada cancelada; cobrança pode ocorrer')
            raise
        finally:
            self.budget.elapsed(time.monotonic() - started)
            self.budget.settle(reserved, cost)
            if not priced:
                self.store.set('cost_unavailable', True)
            elif self.store.get('cost_unavailable') is None:
                self.store.set('cost_unavailable', False)
            if self.client is None:
                await client.aclose()

    async def _read(self, response):
        content, calls, usage, data = [], {}, None, []
        done, finished, size = False, False, 0

        def consume(raw):
            nonlocal done, finished, usage
            if raw == '[DONE]':
                done = True
                return
            try:
                chunk = json.loads(raw)
                require(isinstance(chunk, dict) and 'error' not in chunk, 'Evento de API inválido')
                if chunk.get('usage') is not None:
                    usage = chunk['usage']
                    require(isinstance(usage, dict), 'Uso inválido')
                    for field in ('prompt_tokens', 'completion_tokens'):
                        require(type(usage.get(field)) is int and 0 <= usage[field] <= 1_000_000_000, 'Uso inválido')
                for choice in chunk.get('choices', []):
                    require(choice.get('index', 0) == 0, 'Mais de uma escolha não suportada')
                    reason = choice.get('finish_reason')
                    if reason:
                        require(reason in {'stop', 'tool_calls'}, f'Resposta não concluída: {reason}')
                        finished = True
                    delta = choice.get('delta', {})
                    if delta.get('content'):
                        require(isinstance(delta['content'], str), 'Conteúdo inválido')
                        content.append(delta['content'])
                    for call in delta.get('tool_calls', []):
                        index = call['index']
                        require(type(index) is int and 0 <= index < 16, 'Índice de ferramenta inválido')
                        item = calls.setdefault(index, {'id': '', 'type': 'function',
                                                       'function': {'name': '', 'arguments': ''}})
                        if call.get('id'):
                            item['id'] += call['id']
                        function = call.get('function', {})
                        for field in ('name', 'arguments'):
                            if function.get(field):
                                item['function'][field] += function[field]
            except (json.JSONDecodeError, TypeError, KeyError, AttributeError) as exc:
                raise KrakenError('Streaming inválido; nenhuma ferramenta parcial foi executada') from exc

        async for line in response.aiter_lines():
            size += len(line)
            require(size <= 1_000_000, 'Streaming excedeu limite de tamanho')
            if not line:
                if data:
                    consume('\n'.join(data))
                    data.clear()
                if done:
                    break
            elif line.startswith('data:'):
                data.append(line[5:].lstrip(' '))
        if data:
            consume('\n'.join(data))
        require(done and finished, 'Streaming incompleto; nenhuma ferramenta foi executada')
        result = [calls[i] for i in sorted(calls)]
        require(len({c['id'] for c in result}) == len(result), 'IDs de ferramentas duplicados')
        for call in result:
            require(bool(call['id']) and bool(call['function']['name']), 'Ferramenta sem identidade')
            try:
                require(isinstance(json.loads(call['function']['arguments']), dict), 'Argumentos precisam ser objeto')
            except json.JSONDecodeError as exc:
                raise KrakenError('Argumentos de ferramenta inválidos') from exc
        return Reply(''.join(content), result, usage)

    async def probe(self, role):
        reply = await self.complete(role, [{'role': 'user', 'content':
                    'Call capability_check with ok=true. Do not perform any other action.'}],
                    [{'type': 'function', 'function': {'name': 'capability_check',
                      'description': 'Local in-memory protocol check; no side effects.',
                      'parameters': {'type': 'object', 'properties': {'ok': {'type': 'boolean'}},
                                     'required': ['ok'], 'additionalProperties': False}}}],
                    {'type': 'function', 'function': {'name': 'capability_check'}})
        require(len(reply.tool_calls) == 1 and reply.tool_calls[0]['function']['name'] == 'capability_check'
                and json.loads(reply.tool_calls[0]['function']['arguments']) == {'ok': True},
                'Provedor não demonstrou tool calling em streaming')
        self.store.emit('capability_verified', role=role.name, model=role.profile.model)
