# Worktrunk Inspirasjon - Plan for Gren

> Analyse av [max-sixty/worktrunk](https://github.com/max-sixty/worktrunk) og hva vi kan adoptere i gren.

## Implementasjonsstatus

| Feature | Status | Commit | Prioritet |
|---------|--------|--------|-----------|
| Forbedret Shell Integration | ✅ Ferdig | `8f22643` | - |
| Execute Flag (-x) | ✅ Ferdig | `48ff616` | - |
| TOML Config Support | ✅ Ferdig | `11828b0` | - |
| Extended Hooks System | ✅ Ferdig | `a4d25d2` | - |
| Claude Code Plugin | ✅ Ferdig | `e6b67b2` | - |
| Branch-basert Adressering | ✅ Ferdig | `489008f` | - |
| Spesiell Navigasjon (-) | ✅ Ferdig | `a0bb7a8` | - |
| Statusline Command | ✅ Ferdig | `c4931d2` | - |
| **Unified Merge Command** | ✅ Ferdig | `2d2e34d` | - |
| **for-each Command** | ⏳ Pending | - | 🟡 MEDIUM |
| **LLM Commit Messages** | ⏳ Pending | - | 🟡 MEDIUM |
| **CI Status Integration** | ⏳ Pending | - | 🟡 MEDIUM |
| **Progressive CLI Rendering** | ⏳ Pending | - | 🟢 LAV |
| **Dev Server URL Column** | ⏳ Pending | - | 🟢 LAV |

---

## TL;DR

Worktrunk er en Rust-basert CLI for git worktree management, designet for parallelle AI-agenter. De har løst mange av de samme problemene som gren, men med noen elegante løsninger vi bør vurdere.

**Forskjell mellom gren og worktrunk:** Gren har TUI i tillegg til CLI. Alle features må støtte begge interfaces.

---

## Gjenværende Features å Implementere

### 1. Unified Merge Command (Prioritet: 🔴 HØY)

**DEN VIKTIGSTE FEATUREN.** En kommando som gjør hele merge-workflowen:

```bash
gren merge [target] [--squash] [--no-remove] [--no-verify]
```

**Pipeline (fra worktrunk):**
1. **Stage** - Stage uncommitted changes
2. **Squash** - Kombiner alle commits siden target til én (med LLM-generert melding)
3. **Rebase** - Rebase onto target hvis behind
4. **Pre-merge hooks** - Kjør tests, lint (fail-fast)
5. **Merge** - Fast-forward merge til target branch
6. **Pre-remove hooks** - Kjør cleanup hooks
7. **Cleanup** - Slett worktree + branch
8. **Post-merge hooks** - Kjør deploy, notifications

**Flags:**
- `--no-squash` - Behold individuelle commits
- `--no-remove` - Behold worktree etter merge
- `--no-verify` - Skip hooks
- `--no-rebase` - Skip rebase (fail hvis ikke allerede rebased)

**TUI-integrasjon:**
- Ny keybind `m` for merge current worktree
- Modal confirmation med pipeline preview
- Progress indicator under merge

**Implementasjon:** Stort arbeid. Ny kommando + TUI view.

---

### 2. for-each Command (Prioritet: 🟡 MEDIUM)

Kjør kommando i ALLE worktrees:

```bash
gren for-each -- git status --short
gren for-each -- npm install
gren for-each -- "echo Branch: {{ branch }}"
```

**Template-variabler (fra worktrunk):**
| Variable | Beskrivelse |
|----------|-------------|
| `{{ branch }}` | Branch-navn |
| `{{ branch \| sanitize }}` | Branch-navn med `/` → `-` |
| `{{ worktree }}` | Absolutt path til worktree |
| `{{ worktree_name }}` | Worktree directory name |
| `{{ repo }}` | Repository name |
| `{{ repo_root }}` | Absolutt path til main repo |
| `{{ commit }}` | Full HEAD commit SHA |
| `{{ short_commit }}` | Short HEAD commit SHA |
| `{{ default_branch }}` | Default branch (main/master) |

**Behavior:**
- Kjører sekvensielt i hver worktree
- Fortsetter ved feil, viser summary til slutt
- Real-time output

**TUI-integrasjon:**
- Keybind `F` for "for-each" med input prompt
- Eller via tools menu (`t`)

**Implementasjon:** Medium arbeid. Template-system + command runner.

---

### 3. LLM Commit Messages (Prioritet: 🟡 MEDIUM)

Generer commit-meldinger med ekstern LLM-kommando:

```bash
# Standalone
gren step commit              # Stage + commit med LLM-melding
gren step squash [target]     # Squash med LLM-melding

# Som del av merge
gren merge                    # Bruker LLM for squash-commit
```

**Config (.gren/config.toml):**
```toml
[commit-generation]
command = "llm"
args = ["-m", "claude-haiku-4.5"]
# Eller:
# command = "aichat"
# args = ["-m", "claude"]
```

**Hvordan det fungerer:**
1. Generer diff
2. Bygg prompt med diff + kontekst
3. Pipe til ekstern LLM-kommando
4. Bruk output som commit-melding

**Viktig:** Vi bygger IKKE LLM inn i gren. Vi kaller et eksternt CLI-verktøy (`llm`, `aichat`, etc.).

**Implementasjon:** Medium arbeid. Prompt-template + subprocess.

---

### 4. CI Status Integration (Prioritet: 🟡 MEDIUM)

Vis CI/pipeline-status i list og TUI:

```
Branch        Status   CI   Path
main          ^        ●    ~/code/project
feat-auth     ↑2       ●    ~/code/project.feat-auth
feat-ui       !?       ⚠    ~/code/project.feat-ui
```

**CI-indikatorer:**
| Symbol | Farge | Betydning |
|--------|-------|-----------|
| `●` | Grønn | Alle checks passed |
| `●` | Blå | Checks running |
| `●` | Rød | Checks failed |
| `●` | Gul | Merge conflicts |
| `●` | Grå | No checks configured |
| `⚠` | Gul | Fetch error |

**Implementasjon:**
- Bruk eksisterende `gh` CLI integrasjon
- Cache resultater i git config med TTL (30-60 sek)
- Async fetch - vis tabell først, fyll inn CI etterpå

**TUI-integrasjon:**
- Ny kolonne i dashboard
- Keybind for å åpne PR/pipeline i browser

**Implementasjon:** Medium arbeid. Utvid eksisterende GitHub-integrasjon.

---

### 5. Progressive CLI Rendering (Prioritet: 🟢 LAV)

`gren list` viser lokale data instant, fyller inn remote-data progressivt:

```bash
$ gren list
# Instant: branches, paths, local status
# 100ms later: remote status, commits ahead/behind
# 500ms later: CI status
```

**Hvordan:**
1. Detect om stdout er TTY
2. Hvis TTY: progressive rendering
3. Hvis pipe: buffer alt først

**TUI:** Ikke nødvendig - TUI har allerede async loading.

**Implementasjon:** Lett-medium arbeid. Kun for CLI `list` kommando.

---

### 6. Dev Server URL Column (Prioritet: 🟢 LAV)

Vis dev server URLs i list, dimmet hvis port ikke lytter:

```
Branch        URL                        Path
feat-auth     http://localhost:12472     ~/code/project.feat-auth
feat-ui       http://localhost:13891     ~/code/project.feat-ui
```

**Config (.gren/config.toml):**
```toml
[list]
url = "http://localhost:{{ branch | hash_port }}"
```

**Filters:**
- `hash_port` - Hash string til port 10000-19999
- `sanitize` - Erstatt `/` og `\` med `-`

**Implementasjon:** Lett arbeid, men nisje use case.

---

## Tidligere Implementerte Features

### Forbedret Shell Integration ✅
Mktemp + env var approach for shell directives.

### Execute Flag (-x) ✅
`gren create -n feat -x claude` starter Claude etter worktree-opprettelse.

### TOML Config Support ✅
Støtte for `.gren/config.toml` i tillegg til JSON.

### Extended Hooks System ✅
`pre-remove` hook + backward compat for legacy `PostCreateHook`.

### Claude Code Plugin ✅
`gren marker set/clear/get/list` + `gren setup-claude-plugin`.

### Branch-basert Adressering ✅
`gren switch feat-auth` med fuzzy matching.

### Spesiell Navigasjon (-) ✅
`gren switch -` for forrige worktree.

### Statusline Command ✅
`gren statusline` for shell prompts.

---

## Implementasjonsplan

### Fase 5: Merge Workflow (Neste)
1. **Unified Merge Command**
   - Implementer full pipeline
   - Integrer med eksisterende hooks
   - TUI merge view

2. **LLM Commit Messages**
   - Prompt-template system
   - Ekstern kommando-integrasjon
   - Integrer med merge

### Fase 6: Multi-Worktree Operations
3. **for-each Command**
   - Template-variabler
   - Parallel/sekvensiell execution
   - Error handling + summary

### Fase 7: Status & Monitoring
4. **CI Status Integration**
   - Utvid `gh` integrasjon
   - Caching med TTL
   - TUI + CLI display

5. **Progressive CLI Rendering**
   - TTY detection
   - Streaming output
   - Graceful degradation

### Fase 8: Polish (Valgfritt)
6. **Dev Server URL Column**
   - Port hashing
   - Health check
   - Display i list/TUI

---

## Tekniske Notater

### Merge Command - Detaljert Design

```go
type MergeOptions struct {
    Target      string // Target branch (default: main)
    Squash      bool   // Squash commits (default: true)
    Remove      bool   // Remove worktree after (default: true)
    Verify      bool   // Run hooks (default: true)
    Rebase      bool   // Rebase onto target (default: true)
}

func (wm *WorktreeManager) Merge(ctx context.Context, opts MergeOptions) error {
    // 1. Stage uncommitted changes
    // 2. Squash commits (if opts.Squash)
    // 3. Rebase onto target (if opts.Rebase)
    // 4. Run pre-merge hooks (if opts.Verify)
    // 5. Fast-forward merge
    // 6. Run pre-remove hooks (if opts.Verify && opts.Remove)
    // 7. Remove worktree (if opts.Remove)
    // 8. Switch to target
    // 9. Run post-merge hooks (if opts.Verify)
}
```

### LLM Integration - Design

```go
type CommitGenerator struct {
    Command string   // e.g., "llm"
    Args    []string // e.g., ["-m", "claude-haiku"]
}

func (cg *CommitGenerator) Generate(diff string, context string) (string, error) {
    prompt := buildPrompt(diff, context)
    cmd := exec.Command(cg.Command, cg.Args...)
    cmd.Stdin = strings.NewReader(prompt)
    output, err := cmd.Output()
    return strings.TrimSpace(string(output)), err
}
```

### for-each - Template System

```go
type TemplateContext struct {
    Branch        string
    BranchSanitized string
    Worktree      string
    WorktreeName  string
    Repo          string
    RepoRoot      string
    Commit        string
    ShortCommit   string
    DefaultBranch string
}

func expandTemplate(template string, ctx TemplateContext) string {
    // Simple {{ variable }} replacement
    // Support for {{ variable | filter }}
}
```

---

## Ressurser

- [Worktrunk GitHub](https://github.com/max-sixty/worktrunk)
- [Worktrunk Docs](https://worktrunk.dev)
- [Worktrunk CLI Source](https://github.com/max-sixty/worktrunk/blob/main/src/cli.rs)
- [Claude Code Hooks](https://docs.anthropic.com/claude-code/hooks)
