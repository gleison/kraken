"""Bounded subprocess output; never execute model-provided commands on the host."""
import asyncio
import os
import signal
from dataclasses import dataclass, field

from .domain import KrakenError


@dataclass
class Result:
    code: int
    stdout: str
    stderr: str
    truncated: bool = False
    artifacts: dict[str, str] = field(default_factory=dict)


async def process(argv, *, cwd=None, timeout=120, env=None, limit=64_000, input=None):
    creation = asyncio.create_task(asyncio.create_subprocess_exec(*argv, cwd=cwd, env=env,
             start_new_session=True,
             stdin=asyncio.subprocess.PIPE if input is not None else asyncio.subprocess.DEVNULL,
             stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE))
    try:
        # Preserve ownership of the process even if cancellation arrives during spawn.
        proc = await asyncio.shield(creation)
    except asyncio.CancelledError:
        proc = await creation
        if proc.returncode is None:
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        await proc.wait()
        raise
    truncated = False

    async def read(stream):
        nonlocal truncated
        chunks, size = [], 0
        while chunk := await stream.read(8192):
            available = max(0, limit - size)
            if available:
                chunks.append(chunk[:available])
            size += len(chunk)
            truncated |= size > limit
        return b''.join(chunks).decode(errors='replace')

    async def communicate():
        async def write():
            if input is not None:
                try:
                    proc.stdin.write(input)
                    await proc.stdin.drain()
                except (BrokenPipeError, ConnectionResetError):
                    pass  # Preserve the command's real exit status and stderr.
                finally:
                    proc.stdin.close()
        _, out, err = await asyncio.gather(write(), read(proc.stdout), read(proc.stderr))
        await proc.wait()
        return Result(proc.returncode, out, err, truncated)
    try:
        return await asyncio.wait_for(communicate(), timeout=timeout)
    except (TimeoutError, asyncio.CancelledError):
        if proc.returncode is None:
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        await proc.wait()
        raise
