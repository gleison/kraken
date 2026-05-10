#!/usr/bin/env bash
#
# kraken.sh — autonomous task runner for projects using claude-code-template.
#
# Reads .tasks/BACKLOG.md, picks the next pending task whose dependencies are
# satisfied, and drives it through the start → implement → done lifecycle by
# invoking the `claude` CLI in non-interactive (--print) mode. Repeats until no
# unblocked task remains.
#
# Configuration via env vars:
#   KRAKEN_CLAUDE_BIN     path to the claude CLI (default: claude)
#   KRAKEN_CLAUDE_FLAGS   extra flags passed to every claude invocation
#                           (default: --dangerously-skip-permissions)
#   KRAKEN_MAX_TASKS      stop after this many tasks (default: 0 = no limit)
#   KRAKEN_BRANCH         override the auto-generated branch name
#   KRAKEN_NO_BRANCH=1    skip branch creation (run on current branch)
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR"
BACKLOG_FILE="$REPO_ROOT/.tasks/BACKLOG.md"
COMMANDS_DIR="$REPO_ROOT/.claude/commands"

CLAUDE_BIN="${KRAKEN_CLAUDE_BIN:-claude}"
# WARNING: --dangerously-skip-permissions bypassa todas as confirmações do
# claude (edição de arquivos, comandos shell, escrita em rede, etc.). Use
# apenas em ambientes de desenvolvimento isolados (sandbox, container,
# worktree descartável). Em ambientes compartilhados ou com dados sensíveis,
# substitua por algo como KRAKEN_CLAUDE_FLAGS="--permission-mode acceptEdits".
CLAUDE_FLAGS="${KRAKEN_CLAUDE_FLAGS:---dangerously-skip-permissions}"
MAX_TASKS="${KRAKEN_MAX_TASKS:-0}"

# Force a byte-oriented locale so awk/grep don't choke on UTF-8 sequences
# (em-dash, accented chars, etc.) when parsing the backlog. The default macOS
# BSD awk fails with "multibyte conversion error" otherwise.
export LC_ALL=C

if [ -t 1 ]; then
    C_RESET=$'\033[0m'; C_BOLD=$'\033[1m'
    C_BLUE=$'\033[34m'; C_GREEN=$'\033[32m'; C_RED=$'\033[31m'; C_YELLOW=$'\033[33m'
else
    C_RESET=""; C_BOLD=""; C_BLUE=""; C_GREEN=""; C_RED=""; C_YELLOW=""
fi

log()   { printf '%s[kraken]%s %s\n' "$C_BLUE" "$C_RESET" "$*" >&2; }
ok()    { printf '%s[kraken]%s %s%s%s\n' "$C_BLUE" "$C_RESET" "$C_GREEN" "$*" "$C_RESET" >&2; }
warn()  { printf '%s[kraken]%s %s%s%s\n' "$C_BLUE" "$C_RESET" "$C_YELLOW" "$*" "$C_RESET" >&2; }
fail()  { printf '%s[kraken]%s %s%s%s\n' "$C_BLUE" "$C_RESET" "$C_RED" "$*" "$C_RESET" >&2; }
die()   { fail "error: $*"; exit 1; }

# --- prerequisites ----------------------------------------------------------

command -v "$CLAUDE_BIN" >/dev/null 2>&1 \
    || die "claude CLI not found (looked for: $CLAUDE_BIN)"
[ -f "$BACKLOG_FILE" ] \
    || die "missing $BACKLOG_FILE — run /task-spec first to generate the backlog"
[ -f "$COMMANDS_DIR/task-start.md" ] \
    || die "missing $COMMANDS_DIR/task-start.md"
[ -f "$COMMANDS_DIR/task-done.md" ] \
    || die "missing $COMMANDS_DIR/task-done.md"
git -C "$REPO_ROOT" rev-parse --git-dir >/dev/null 2>&1 \
    || die "$REPO_ROOT is not a git repository"

# --- branch ----------------------------------------------------------------

if [ "${KRAKEN_NO_BRANCH:-0}" = "1" ]; then
    BRANCH=$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD)
    log "running on current branch: $BRANCH"
else
    TIMESTAMP=$(date -u +%Y%m%d-%H%M%S)
    BRANCH="${KRAKEN_BRANCH:-kraken/run-$TIMESTAMP}"
    log "creating branch $BRANCH"
    git -C "$REPO_ROOT" checkout -b "$BRANCH"
fi

# --- backlog parsing -------------------------------------------------------

list_pending_task_ids() {
    grep -oE '^### \[ \] TASK-[0-9]+' "$BACKLOG_FILE" \
        | grep -oE 'TASK-[0-9]+' || true
}

is_task_done_in_backlog() {
    grep -qE "^### \[x\] $1([[:space:]]|\$)" "$BACKLOG_FILE"
}

extract_task_block() {
    awk -v id="$1" '
        function start_of_section(line) {
            return line ~ /^## / || line ~ /^### /
        }
        {
            if ($0 ~ /^### \[.\] TASK-[0-9]+/) {
                line = $0
                sub(/^### \[.\] /, "", line)
                split(line, w, " ")
                if (in_task) exit
                if (w[1] == id) { in_task = 1; print; next }
            } else if ($0 ~ /^## / && in_task) {
                exit
            }
            if (in_task) print
        }
    ' "$BACKLOG_FILE"
}

extract_task_title() {
    awk -v id="$1" '
        match($0, "^### \\[.\\] " id "([[:space:]]|$)") {
            rest = substr($0, RSTART + RLENGTH - 1)
            # consume leading bytes that are not alphanumeric (whitespace,
            # ASCII hyphen, en-dash and em-dash bytes, etc.)
            while (length(rest) > 0) {
                first = substr(rest, 1, 1)
                if (first ~ /[[:alnum:]]/) break
                rest = substr(rest, 2)
            }
            print rest
            exit
        }
    ' "$BACKLOG_FILE"
}

extract_dep_ids() {
    extract_task_block "$1" \
        | grep -E '^- \*\*Depende de:\*\*' \
        | grep -oE 'TASK-[0-9]+' \
        | sort -u || true
}

find_next_available_task() {
    local id dep all_done
    while IFS= read -r id; do
        [ -z "$id" ] && continue
        all_done=true
        while IFS= read -r dep; do
            [ -z "$dep" ] && continue
            if ! is_task_done_in_backlog "$dep"; then
                all_done=false
                break
            fi
        done < <(extract_dep_ids "$id")
        if $all_done; then
            printf '%s\n' "$id"
            return 0
        fi
    done < <(list_pending_task_ids)
    return 1
}

mark_task_done_in_backlog() {
    local id="$1"
    awk -v id="$id" '
        $0 ~ "^### \\[ \\] "id"([[:space:]]|$)" {
            sub(/\[ \]/, "[x]")
        }
        { print }
    ' "$BACKLOG_FILE" > "$BACKLOG_FILE.tmp"
    mv "$BACKLOG_FILE.tmp" "$BACKLOG_FILE"
}

# --- claude invocation -----------------------------------------------------

run_claude() {
    local label="$1"
    local prompt="$2"
    local tmp rc=0
    tmp=$(mktemp "${TMPDIR:-/tmp}/kraken-prompt.XXXXXX")
    printf '%s' "$prompt" > "$tmp"
    log "→ claude ($label)"
    # shellcheck disable=SC2086  # CLAUDE_FLAGS is intentionally word-split
    "$CLAUDE_BIN" $CLAUDE_FLAGS --print "$(cat "$tmp")" || rc=$?
    rm -f "$tmp"
    return $rc
}

# --- main loop -------------------------------------------------------------

COMPLETED=()
FAILED=()
ATTEMPTED=()
RUNS=0

was_attempted() {
    local id="$1" prev
    for prev in ${ATTEMPTED[@]+"${ATTEMPTED[@]}"}; do
        [ "$prev" = "$id" ] && return 0
    done
    return 1
}

while true; do
    if [ "$MAX_TASKS" -gt 0 ] && [ "$RUNS" -ge "$MAX_TASKS" ]; then
        log "reached KRAKEN_MAX_TASKS=$MAX_TASKS, stopping"
        break
    fi

    if ! task_id=$(find_next_available_task); then
        log "no more pending tasks with satisfied dependencies"
        break
    fi
    [ -z "$task_id" ] && break

    if was_attempted "$task_id"; then
        warn "task $task_id was already attempted in this run; aborting to avoid loop"
        FAILED+=("$task_id (loop detected — task remained pending after run)")
        break
    fi
    ATTEMPTED+=("$task_id")
    RUNS=$((RUNS + 1))

    task_title=$(extract_task_title "$task_id")
    task_block=$(extract_task_block "$task_id")

    printf '\n%s========================================%s\n' "$C_BOLD" "$C_RESET" >&2
    log "starting $task_id — $task_title"
    printf '%s========================================%s\n\n' "$C_BOLD" "$C_RESET" >&2

    # Phase 1 — declare contract via task-start
    start_template=$(cat "$COMMANDS_DIR/task-start.md")
    prompt_start="$start_template

---
Contexto da tarefa atual ($task_id — $task_title):

$task_block
---

Use o bloco acima como base. Não aguarde minha confirmação — escreva
\`.tasks/CURRENT.md\` com o contrato e prossiga."

    if ! run_claude "task-start: $task_id" "$prompt_start"; then
        FAILED+=("$task_id (task-start failed)")
        break
    fi

    # Phase 2 — implementation
    prompt_impl="Siga as diretrizes de AGENTS.md e DECISIONS.md do projeto.

Implemente integralmente a tarefa $task_id descrita abaixo.
Respeite o escopo, o que está fora de escopo, o critério de conclusão e os
arquivos prováveis. Continue trabalhando até cobrir todos os critérios de
conclusão. Não marque a tarefa como concluída no BACKLOG.md ainda — isso
acontecerá na fase de validação. Não faça commits ainda.

$task_block"

    if ! run_claude "implement: $task_id" "$prompt_impl"; then
        FAILED+=("$task_id (implementation failed)")
        break
    fi

    # Phase 3 — validation via task-done
    done_template=$(cat "$COMMANDS_DIR/task-done.md")
    prompt_done="$done_template

---
Tarefa em validação: $task_id — $task_title

$task_block
---"

    if ! run_claude "task-done: $task_id" "$prompt_done"; then
        FAILED+=("$task_id (task-done failed)")
        break
    fi

    # Make sure the backlog reflects completion (task-done usually does this,
    # but we enforce it so the loop always advances).
    if ! is_task_done_in_backlog "$task_id"; then
        warn "$task_id still marked [ ] after task-done; marking [x] to advance"
        mark_task_done_in_backlog "$task_id"
    fi

    # Commit any leftover changes. task-done normally commits already, in
    # which case there will be nothing to stage.
    git -C "$REPO_ROOT" add -A
    if git -C "$REPO_ROOT" diff --cached --quiet; then
        log "no extra changes to commit for $task_id (task-done already committed)"
    else
        commit_msg="$task_id — $task_title

Committed by kraken.sh"
        if ! git -C "$REPO_ROOT" commit -m "$commit_msg"; then
            FAILED+=("$task_id (commit failed)")
            break
        fi
    fi

    ok "completed $task_id"
    COMPLETED+=("$task_id — $task_title")
done

# --- summary ---------------------------------------------------------------

remaining=$(list_pending_task_ids | wc -l | tr -d ' ')

printf '\n%s================ KRAKEN SUMMARY ================%s\n' "$C_BOLD" "$C_RESET" >&2
log "branch:    $BRANCH"
log "completed: ${#COMPLETED[@]}"
for t in ${COMPLETED[@]+"${COMPLETED[@]}"}; do
    printf '   %s✓%s %s\n' "$C_GREEN" "$C_RESET" "$t" >&2
done
log "failed:    ${#FAILED[@]}"
for t in ${FAILED[@]+"${FAILED[@]}"}; do
    printf '   %s✗%s %s\n' "$C_RED" "$C_RESET" "$t" >&2
done
log "pending tasks remaining in backlog: $remaining"
printf '%s==================================================%s\n\n' "$C_BOLD" "$C_RESET" >&2

if [ "${#FAILED[@]}" -gt 0 ]; then
    exit 1
fi
exit 0
