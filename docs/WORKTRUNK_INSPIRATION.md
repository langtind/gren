# Worktrunk Inspirasjon - Plan for Gren

> Analyse av [max-sixty/worktrunk](https://github.com/max-sixty/worktrunk) og hva vi kan adoptere i gren.

## TL;DR

Worktrunk er en Rust-basert CLI for git worktree management, designet for parallelle AI-agenter. De har løst mange av de samme problemene som gren, men med noen elegante løsninger vi bør vurdere.

---

## Funksjoner vi BØR adoptere

### 1. Forbedret Shell Integration (Prioritet: HØY)

**Nåværende gren-løsning:**
```bash
# Fast temp-fil, kun for navigasjon
local TEMP_FILE="/tmp/gren_navigate"
```

**Worktrunk sin løsning (bedre):**
```bash
gren() {
    local directive_file="$(mktemp)"
    GREN_DIRECTIVE_FILE="$directive_file" command gren "$@" || exit_code=$?
    if [[ -s "$directive_file" ]]; then
        source "$directive_file"
    fi
    rm -f "$directive_file"
}
```

**Hvorfor bedre:**
- `mktemp` = unike filer, ingen race conditions
- Env var = binæren vet hvor den skal skrive
- Mer fleksibelt = kan skrive hvilke som helst shell-kommandoer
- Støtter fremtidige features som automatisk cd etter create

**Implementasjon:** Medium arbeid. Endre `shell-init` output og `navigate` kommando.

---

### 2. Execute Flag `-x` (Prioritet: HØY)

Start en kommando etter worktree-operasjoner:

```bash
# Lag worktree OG start Claude
gren create -n feat-auth -x claude

# Switch og start dev server
gren switch feat-ui -x "npm run dev"

# Med trailing args (etter --)
gren create -n feat -x claude -- "implement login"
```

**Hvorfor:**
- Perfekt for AI-agent workflows
- Reduserer friction dramatisk
- Enkel å implementere med den nye shell-integrasjonen

**Implementasjon:** Lett arbeid. Legg til `-x` flag, skriv kommando til directive file.

---

### 3. Claude Code Plugin (Prioritet: HØY)

Aktivitetstracking for parallelle Claude-sesjoner:

```
# I gren list/TUI:
  main       ✓   12m
  feat-auth  🤖  +2   # Claude jobber her
  feat-ui    💬  +5   # Venter på input
```

**Implementasjon:**
1. `.claude-plugin/` directory med hooks
2. Hooks setter markers via git config
3. TUI/list leser markers og viser status

```json
// .claude-plugin/hooks/hooks.json
{
  "hooks": {
    "UserPromptSubmit": [{ "command": "gren marker set working" }],
    "Notification": [{ "command": "gren marker set waiting" }],
    "SessionEnd": [{ "command": "gren marker clear" }]
  }
}
```

**Implementasjon:** Medium arbeid. Ny `gren marker` kommando + oppdater TUI.

---

### 4. Utvidet Hooks System (Prioritet: MEDIUM)

Worktrunk har 7 hook-typer. Gren har kun `post-create`.

**Nye hooks å legge til:**
| Hook | Når | Use Case |
|------|-----|----------|
| `post-start` | Etter switch (bakgrunn) | Dev servers, file watchers |
| `post-switch` | Etter alle switches | IDE/terminal updates |
| `pre-merge` | Før merge | Tests, lint |
| `post-merge` | Etter merge | Cleanup, deploy |
| `pre-remove` | Før sletting | Backup, verification |

**Config-format:**
```toml
# .gren/config.toml (eller .config/gren.toml)
post-create = "bun install"

[pre-merge]
lint = "bun run lint"
test = "bun test"

[post-start]
dev = { command = "bun run dev", background = true }
```

**Implementasjon:** Medium-stort arbeid. Utvide config-system, hook-runner.

---

### 5. Unified Merge Command (Prioritet: MEDIUM)

En kommando som gjør hele merge-workflowen:

```bash
gren merge [--squash] [--no-remove]
```

Gjør automatisk:
1. Commit/squash endringer
2. Rebase onto target
3. Kjør pre-merge hooks (tests)
4. Push til target branch
5. Slett worktree + branch
6. Switch til main
7. Kjør post-merge hooks

**Hvorfor:**
- Eliminerer manuelt arbeid
- Konsistent workflow
- Hooks sikrer kvalitet

**Implementasjon:** Stort arbeid. Ny kommando med mange steg.

---

### 6. Branch-basert Adressering (Prioritet: MEDIUM)

Adresser worktrees via branch-navn, ikke path:

```bash
# I stedet for:
gren switch /path/to/repo-worktrees/feat-auth

# Bare:
gren switch feat-auth
```

Med path-templates:
```toml
[worktree]
path = "../{{ repo }}-worktrees/{{ branch | sanitize }}"
```

**Implementasjon:** Medium arbeid. Template-system + lookup-logikk.

---

### 7. Spesiell Navigasjon (Prioritet: LAV)

```bash
gren switch -      # Forrige worktree (som cd -)
gren switch @      # Current (noop, men nyttig i scripts)
```

**Implementasjon:** Lett arbeid. Track previous i git config eller env var.

---

### 8. Status Line for Shell Prompts (Prioritet: LAV)

```bash
# I .zshrc
PROMPT='$(gren statusline) %~ $ '

# Output: feat-auth +2 ↑1
```

**Implementasjon:** Medium arbeid. Ny kommando med rask git-status.

---

## Funksjoner vi IKKE bør adoptere

### 1. LLM Commit Messages
**Hvorfor ikke:**
- Scope creep - gren er en worktree manager, ikke commit helper
- Legger til kompleksitet og dependencies
- Brukere kan bruke dedikerte verktøy (llm, aichat, etc.)
- Claude Code genererer allerede commit messages

### 2. CI Status Integration
**Hvorfor ikke:**
- Gren har allerede GitHub CLI integrasjon for PR-status
- Legger til API-avhengigheter og kompleksitet
- CI-status er tilgjengelig andre steder

### 3. Progressive List Rendering
**Hvorfor ikke:**
- Gren er TUI-first, ikke CLI-first
- TUI har allerede async loading
- Ville kreve stor refaktorering

### 4. for-each Command
**Hvorfor ikke:**
- Nisje use case
- Enkelt å gjøre med shell-løkker
- Lav verdi vs kompleksitet

### 5. Dev Server URL Column
**Hvorfor ikke:**
- Veldig nisje
- Krever template-system og health checks
- Lav prioritet

---

## Implementasjonsplan

### Fase 1: Fundament (1-2 uker arbeid)
1. **Forbedret Shell Integration**
   - Endre til env var + mktemp approach
   - Oppdater alle shells (bash, zsh, fish)
   - Bakoverkompatibilitet med eksisterende setup

2. **Execute Flag**
   - Legg til `-x` på `create` og evt. `switch`
   - Integrer med directive file system

### Fase 2: Claude Integration (1 uke arbeid)
3. **Claude Code Plugin**
   - Opprett `.claude-plugin/` struktur
   - Implementer `gren marker` kommando
   - Oppdater TUI til å vise markers
   - Dokumenter oppsett

### Fase 3: Workflow (2-3 uker arbeid)
4. **Utvidet Hooks System**
   - Design config-format
   - Implementer hook-runner med parallelitet
   - Legg til nye hook-typer gradvis

5. **Branch-basert Adressering**
   - Template-system for paths
   - Lookup-logikk
   - Migrering av eksisterende worktrees

### Fase 4: Polish (1 uke arbeid)
6. **Spesiell Navigasjon**
   - `-` for previous worktree

7. **Merge Command** (valgfritt)
   - Unified workflow
   - Hook-integrasjon

---

## Tekniske Notater

### Shell Integration - Detaljert Design

```bash
# Ny zsh wrapper
gren() {
    # Skip wrapper i completion mode
    if [[ -n "${COMPLETE:-}" ]]; then
        command gren "$@"
        return
    fi

    local directive_file exit_code=0
    directive_file="$(mktemp)"

    GREN_DIRECTIVE_FILE="$directive_file" command gren "$@" || exit_code=$?

    if [[ -s "$directive_file" ]]; then
        source "$directive_file"
        if [[ $exit_code -eq 0 ]]; then
            exit_code=$?
        fi
    fi

    rm -f "$directive_file"
    return "$exit_code"
}
```

Go-kode for å skrive directives:
```go
func WriteDirective(directive string) error {
    file := os.Getenv("GREN_DIRECTIVE_FILE")
    if file == "" {
        return nil // Shell integration ikke aktiv
    }
    return os.WriteFile(file, []byte(directive+"\n"), 0644)
}

// Bruk:
WriteDirective(fmt.Sprintf("cd %q", worktreePath))
WriteDirective(fmt.Sprintf("exec %s", executeCommand))
```

### Claude Plugin - Detaljert Design

Directory struktur:
```
.claude-plugin/
├── plugin.json
├── hooks/
│   └── hooks.json
└── skills/
    └── gren/
        └── SKILL.md
```

Marker storage (git config):
```bash
# Set marker
git config --local gren.marker.feat-auth "🤖"

# Get marker
git config --local gren.marker.feat-auth

# Clear
git config --local --unset gren.marker.feat-auth
```

---

## Spørsmål å avklare

1. **Config-format:** Fortsette med JSON eller gå over til TOML?
   - TOML er mer lesbart for hooks
   - Worktrunk bruker TOML

2. **Backward compatibility:** Hvordan håndtere eksisterende `.gren/` configs?
   - Migreringsscript?
   - Støtte begge formater?

3. **Plugin publisering:** Skal claude-plugin være i gren-repo eller separat?
   - I repo = enklere vedlikehold
   - Separat = kan oppdateres uavhengig

4. **Navnekonvensjoner:** Beholde `gren` eller alias som `gw` for CLI?

---

## Ressurser

- [Worktrunk GitHub](https://github.com/max-sixty/worktrunk)
- [Worktrunk Docs](https://worktrunk.dev)
- [Claude Code Hooks](https://docs.anthropic.com/claude-code/hooks)
- [Anthropic Blog: Claude Code Best Practices](https://www.anthropic.com/engineering/claude-code-best-practices)
