# Kraken

Aautonomous task runner for projects using claude-code-template.

Reads `.tasks/BACKLOG.md`, picks the next pending task whose dependencies are satisfied, and drives it through the start → implement → done lifecycle by invoking the `claude` CLI in non-interactive (--print) mode. Repeats until no unblocked task remains.

## Configuration via env vars:
    
    KRAKEN_CLAUDE_BIN     path to the claude CLI (default: claude)
    KRAKEN_CLAUDE_FLAGS   extra flags passed to every claude invocation (default: --dangerously-skip-permissions)
    KRAKEN_MAX_TASKS      stop after this many tasks (default: 0 = no limit)
    KRAKEN_BRANCH         override the auto-generated branch name
    KRAKEN_NO_BRANCH=1    skip branch creation (run on current branch)

