# Plan: Gren Bedre Enn Worktrunk

> **Mål:** Gren skal ha ALT Worktrunk har, PLUSS TUI som ekstra fordel. Bedre dokumentasjon, bedre brukeropplevelse.

**Dato:** 2025-12-31
**Status:** Planlegging

---

## Nåværende Status

### Gren har som Worktrunk mangler
| Feature | Status | Forbedringspotensial |
|---------|--------|---------------------|
| TUI Dashboard | ✅ | Utvid med mer info |
| Init Wizard | ✅ | Legg til flere opsjoner |
| Visual Git Status | ✅ | Legg til CI-status visning |
| Package Manager Detection | ✅ | Solid |
| Env File Symlinking | ✅ | Solid |
| Missing Worktree Detection | ✅ | Solid |
| Responsive Layout | ✅ | Solid |

### Worktrunk har som Gren mangler
| Feature | Prioritet | Kompleksitet | Status |
|---------|-----------|--------------|--------|
| Hook Approval System | 🔴 Kritisk | Høy | ❌ Mangler |
| Named Hooks | 🔴 Kritisk | Medium | ❌ Mangler |
| post-start Hook | 🔴 Høy | Lav | ❌ Mangler |
| post-switch Hook | 🔴 Høy | Lav | ❌ Mangler |
| Shell Completions | 🟡 Medium | Medium | ❌ Mangler |
| User Config (global) | 🟡 Medium | Medium | ❌ Mangler |
| JSON Context til Hooks | 🟡 Medium | Lav | ❌ Mangler |
| LLM Template System | 🟡 Medium | Medium | ❌ Mangler |
| GitLab Support | 🟡 Medium | Medium | ❌ Mangler |
| Progressive CLI Rendering | 🟢 Lav | Medium | ❌ Mangler |
| Colored Help Output | 🟢 Lav | Lav | ❌ Mangler |
| Markdown/Long Help | 🟢 Lav | Lav | ❌ Mangler |
| Integration Reason Display | 🟢 Lav | Lav | ❌ Mangler |
| Backup Refs før Squash | 🟢 Lav | Lav | ❌ Mangler |
| Lock File Filtering (LLM) | 🟢 Lav | Lav | ❌ Mangler |
| Bare Repo Support | 🟢 Lav | Medium | ❌ Mangler |

---

## Fase 1: Sikkerhet & Hooks (Kritisk)

### 1.1 Hook Approval System
**Hvorfor:** Worktrunk ber om godkjenning før hooks kjøres. Dette er en sikkerhetsfunksjon som forhindrer at ondsinnet kode i `.gren/config.toml` kjøres automatisk.

**Implementasjon:**
```go
type ApprovalManager struct {
    ApprovedCommands map[string]bool // projectID -> command -> approved
    ConfigPath       string          // ~/.config/gren/approved-commands.json
}

func (am *ApprovalManager) RequestApproval(commands []HookCommand, projectID string, autoYes bool) (bool, error) {
    // 1. Sjekk om alle kommandoer allerede er godkjent
    // 2. Hvis ikke, vis liste og spør bruker
    // 3. Lagre godkjente kommandoer permanent
}
```

**TUI-integrasjon:**
- Modal dialog som viser kommandoer før kjøring
- Checkbox for "Alltid godta for dette prosjektet"
- Tydelig visning av hva som vil kjøres

**CLI-integrasjon:**
- `--yes` flag for automatisk godkjenning
- Interaktiv prompt med fargekoding

**Filer å endre:**
- `internal/core/hooks.go` - Legg til approval logic
- `internal/config/approval.go` - Ny fil for approval storage
- `internal/ui/approval.go` - TUI modal for godkjenning
- `internal/cli/approval.go` - CLI prompt

### 1.2 Named Hooks
**Hvorfor:** Worktrunk tillater å navngi individuelle hooks, noe som gjør det lettere å:
- Godkjenne spesifikke hooks
- Kjøre kun visse hooks
- Feilsøke hook-problemer

**Nåværende format (.gren/config.toml):**
```toml
[hooks]
post-create = ["npm install", "npm run dev"]
```

**Nytt format:**
```toml
[[hooks.post-create]]
name = "install-deps"
command = "npm install"

[[hooks.post-create]]
name = "start-dev"
command = "npm run dev"
```

**Bakoverkompatibilitet:** Støtt begge formater. Array-format konverteres internt til named hooks med auto-genererte navn.

**Filer å endre:**
- `internal/config/config.go` - Utvid hook parsing
- `internal/core/hooks.go` - Håndter named hooks

### 1.3 Nye Hook Types

#### post-start Hook
Kjøres etter `gren create` med `-x` flag, når ekstern kommando starter.

```toml
[[hooks.post-start]]
name = "notify-slack"
command = "curl -X POST https://slack.com/api/..."
```

#### post-switch Hook
Kjøres etter `gren switch` til en annen worktree.

```toml
[[hooks.post-switch]]
name = "refresh-env"
command = "direnv reload"
```

**Filer å endre:**
- `internal/core/types.go` - Legg til HookType enum verdier
- `internal/core/hooks.go` - Implementer hook kjøring
- `internal/cli/switch.go` - Kall post-switch
- `internal/cli/create.go` - Kall post-start

---

## Fase 2: Konfigurasjon & UX (Høy Prioritet)

### 2.1 User Config (Global)
**Hvorfor:** Worktrunk har global bruker-konfigurasjon i `~/.config/worktrunk/config.toml`. Dette tillater:
- Standardinnstillinger på tvers av prosjekter
- Personlige preferanser (LLM-kommando, default hooks)

**Lokasjon:**
- macOS: `~/Library/Application Support/gren/config.toml`
- Linux: `~/.config/gren/config.toml`

**Innhold:**
```toml
# User defaults
[defaults]
worktree-dir = "../{{ repo }}-worktrees"
remove-after-merge = true

# LLM configuration
[commit-generation]
command = "llm"
args = ["-m", "claude-haiku-4.5"]

# Global hooks (kjøres for alle prosjekter)
[[hooks.post-create]]
name = "global-notify"
command = "notify-send 'Worktree created'"

# Approved commands (auto-populated)
[approved-commands]
"my-project" = ["npm install", "npm run dev"]
```

**Merge-prioritet:**
1. CLI flags (høyest)
2. Prosjekt-config (`.gren/config.toml`)
3. User config (`~/.config/gren/config.toml`)
4. Defaults (lavest)

**Filer å endre:**
- `internal/config/user_config.go` - Ny fil
- `internal/config/config.go` - Merge user + project config

### 2.2 JSON Context til Hooks
**Hvorfor:** Worktrunk sender JSON-kontekst til hooks via stdin. Dette gir hooks tilgang til all relevant informasjon.

**Eksempel JSON:**
```json
{
  "hook_type": "post-create",
  "branch": "feat-auth",
  "worktree": "/Users/arild/code/project.feat-auth",
  "worktree_name": "project.feat-auth",
  "repo": "project",
  "repo_root": "/Users/arild/code/project",
  "commit": "abc123def456...",
  "short_commit": "abc123d",
  "default_branch": "main",
  "target_branch": "main"
}
```

**Implementasjon:**
```go
type HookContext struct {
    HookType      string `json:"hook_type"`
    Branch        string `json:"branch"`
    Worktree      string `json:"worktree"`
    WorktreeName  string `json:"worktree_name"`
    Repo          string `json:"repo"`
    RepoRoot      string `json:"repo_root"`
    Commit        string `json:"commit"`
    ShortCommit   string `json:"short_commit"`
    DefaultBranch string `json:"default_branch"`
    TargetBranch  string `json:"target_branch,omitempty"`
}

func (ctx HookContext) JSON() ([]byte, error) {
    return json.Marshal(ctx)
}
```

**Filer å endre:**
- `internal/core/hooks.go` - Send JSON til stdin ved hook-kjøring

### 2.3 Shell Completions
**Hvorfor:** Worktrunk har shell completions for bash, zsh, fish. Dette forbedrer CLI-opplevelsen betydelig.

**Implementasjon med cobra:**
```go
// Allerede støttet av cobra!
rootCmd.GenBashCompletion(os.Stdout)
rootCmd.GenZshCompletion(os.Stdout)
rootCmd.GenFishCompletion(os.Stdout)
```

**Ny kommando:**
```bash
gren completion bash > /usr/local/etc/bash_completion.d/gren
gren completion zsh > ~/.zsh/completions/_gren
gren completion fish > ~/.config/fish/completions/gren.fish
```

**Filer å endre:**
- `internal/cli/completion.go` - Ny fil med completion-kommando

---

## Fase 3: LLM & Template System (Medium Prioritet)

### 3.1 LLM Template System
**Hvorfor:** Worktrunk har et sofistikert template-system for LLM-prompts med minijinja.

**Nåværende Gren-implementasjon:**
- Hardkodet prompt-template
- Ingen bruker-tilpasning

**Worktrunk-features vi trenger:**
1. **Tilpassbare templates:**
   ```toml
   [commit-generation]
   template = """
   Write a commit message for these changes.
   Branch: {{ branch }}
   Target: {{ target_branch }}

   Diff:
   {{ diff }}
   """
   ```

2. **Template-filer:**
   ```toml
   [commit-generation]
   template-file = ".gren/commit-template.txt"
   ```

3. **Lock file filtering:**
   ```go
   var lockFiles = []string{
       "package-lock.json",
       "yarn.lock",
       "pnpm-lock.yaml",
       "Cargo.lock",
       "go.sum",
       // etc.
   }

   func filterLockFiles(diff string) string {
       // Fjern lock-filer fra diff for å spare tokens
   }
   ```

4. **Diff size limit:**
   ```go
   const DIFF_SIZE_THRESHOLD = 400_000 // karakterer

   func truncateDiff(diff string) (string, bool) {
       if len(diff) > DIFF_SIZE_THRESHOLD {
           return diff[:DIFF_SIZE_THRESHOLD] + "\n... (truncated)", true
       }
       return diff, false
   }
   ```

**Filer å endre:**
- `internal/core/llm.go` - Utvid med template support
- `internal/config/config.go` - Template config parsing

### 3.2 Backup Refs før Squash
**Hvorfor:** Worktrunk lager backup-referanser før squash slik at man kan angre.

```bash
# Før squash
git update-ref refs/backup/feat-auth HEAD

# Etter squash, hvis noe går galt:
git reset --hard refs/backup/feat-auth
```

**Filer å endre:**
- `internal/core/merge.go` - Legg til backup logic

---

## Fase 4: CI & Integrasjoner (Medium Prioritet)

### 4.1 GitLab Support
**Hvorfor:** Worktrunk støtter både GitHub og GitLab. Gren støtter bare GitHub.

**Implementasjon:**
```go
type CIProvider interface {
    GetPRStatus(branch string) (*PRStatus, error)
    GetCIStatus(branch string) (*CIStatus, error)
    OpenPR(branch string) error
}

type GitHubProvider struct { /* ... */ }
type GitLabProvider struct { /* ... */ }

func DetectProvider(repoURL string) CIProvider {
    if strings.Contains(repoURL, "gitlab") {
        return &GitLabProvider{}
    }
    return &GitHubProvider{}
}
```

**Config:**
```toml
[git]
provider = "gitlab"  # auto-detect hvis ikke satt
gitlab-host = "gitlab.mycompany.com"  # for self-hosted
```

**Filer å endre:**
- `internal/git/provider.go` - Ny fil med provider interface
- `internal/git/github.go` - Refaktorer eksisterende GitHub-kode
- `internal/git/gitlab.go` - Ny GitLab provider

### 4.2 Integration Reason Display
**Hvorfor:** Worktrunk viser hvorfor en branch er integrert (merged, rebased, etc.).

```
✓ feat-auth integrated via merge commit abc123
✓ fix-bug integrated via squash
✓ refactor integrated (branch deleted on remote)
```

**Filer å endre:**
- `internal/core/worktree.go` - Legg til integration detection
- `internal/ui/dashboard.go` - Vis integration reason

---

## Fase 5: CLI Polish (Lav Prioritet)

### 5.1 Colored Help Output
**Hvorfor:** Worktrunk har farget `--help` output som er lettere å lese.

**Implementasjon med lipgloss:**
```go
var (
    helpHeading = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
    helpFlag    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
    helpDesc    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)
```

**Filer å endre:**
- `internal/cli/help.go` - Custom help template

### 5.2 Progressive CLI Rendering
**Hvorfor:** `gren list` viser lokale data instant, fyller inn remote-data progressivt.

**Implementasjon:**
```go
func ListWithProgress() {
    // 1. Vis lokal data umiddelbart
    printLocalData(worktrees)

    // 2. Hvis TTY, start async fetch
    if term.IsTerminal(os.Stdout.Fd()) {
        go func() {
            remoteData := fetchRemoteData(worktrees)
            redrawWithRemoteData(remoteData)
        }()
    }
}
```

**Filer å endre:**
- `internal/cli/list.go` - Progressive rendering

### 5.3 Bare Repo Support
**Hvorfor:** Worktrunk støtter bare repositories (repos uten working directory).

**Filer å endre:**
- `internal/git/repository.go` - Håndter bare repos

---

## Fase 6: Dokumentasjon (Løpende)

### 6.1 README Forbedringer
- [ ] Komplett feature-liste
- [ ] Sammenligning med alternativer
- [ ] Installasjonsinstruksjoner for alle plattformer
- [ ] GIF/video av TUI i bruk
- [ ] Badges (CI, versjon, lisens)

### 6.2 Man Pages
```bash
gren help create    # Detaljert hjelp
gren help hooks     # Hook dokumentasjon
gren help config    # Konfigurasjonsveiledning
```

### 6.3 Eksempelkonfigurasjoner
- [ ] `.gren/config.toml` eksempler for ulike prosjekttyper
- [ ] Hook-eksempler (npm, go, python, rust)
- [ ] LLM-template eksempler

---

## Implementasjonsrekkefølge

### Sprint 1: Sikkerhet (Uke 1-2)
1. ✅ Hook Approval System
2. ✅ Named Hooks
3. ✅ User Config

### Sprint 2: Hooks & UX (Uke 3-4)
4. post-start Hook
5. post-switch Hook
6. JSON Context til Hooks
7. Shell Completions

### Sprint 3: LLM & Templates (Uke 5-6)
8. LLM Template System
9. Lock File Filtering
10. Backup Refs før Squash

### Sprint 4: Integrasjoner (Uke 7-8)
11. GitLab Support
12. Integration Reason Display

### Sprint 5: Polish (Uke 9-10)
13. Colored Help Output
14. Progressive CLI Rendering
15. Bare Repo Support
16. Dokumentasjon

---

## Suksesskriterier

### Gren er bedre enn Worktrunk når:

1. **Feature Paritet:** Alle Worktrunk-features er implementert
2. **TUI Fordel:** TUI gir merverdi som CLI ikke har
3. **Dokumentasjon:** Bedre enn Worktrunk's docs
4. **Brukeropplevelse:** Minst like god som Worktrunk
5. **Ytelse:** Minst like rask som Worktrunk (Rust vs Go)
6. **Sikkerhet:** Hook approval system like robust

### Målbare Metrics:
- [ ] 100% feature paritet med Worktrunk
- [ ] README med >1000 ord dokumentasjon
- [ ] Shell completions for bash, zsh, fish
- [ ] Hook approval med persistent storage
- [ ] GitLab support i tillegg til GitHub
- [ ] LLM template system med bruker-tilpasning

---

## Referanser

- [Worktrunk Source](file:///tmp/worktrunk/)
- [Worktrunk CLI](file:///tmp/worktrunk/src/cli.rs)
- [Worktrunk Merge](file:///tmp/worktrunk/src/commands/merge.rs)
- [Worktrunk LLM](file:///tmp/worktrunk/src/llm.rs)
- [Gren WORKTRUNK_INSPIRATION](file:///Users/arild/Developer/Private/gren/docs/WORKTRUNK_INSPIRATION.md)
