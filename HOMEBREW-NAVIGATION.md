# Navigation med Homebrew

Denne guiden viser hvordan du setter opp og bruker navigation-funksjonen når gren er installert via Homebrew.

## Installasjon

```bash
brew install langtind/tap/gren
```

Etter installasjon vil du se en melding med setup-instruksjoner for navigasjon.

## Setup av Shell Integration

Velg din shell og følg instruksjonene:

### Zsh (macOS standard)
```bash
# Legg til i ~/.zshrc
echo 'eval "$(gren shell-init zsh)"' >> ~/.zshrc

# Reload shell
source ~/.zshrc
```

### Bash
```bash
# For macOS (legg til i ~/.bash_profile)
echo 'eval "$(gren shell-init bash)"' >> ~/.bash_profile
source ~/.bash_profile

# For Linux (legg til i ~/.bashrc)
echo 'eval "$(gren shell-init bash)"' >> ~/.bashrc
source ~/.bashrc
```

### Fish
```bash
# Legg til fish-config
gren shell-init fish >> ~/.config/fish/config.fish

# Restart fish eller kjør
source ~/.config/fish/config.fish
```

## Bruk etter setup

Når shell integration er satt opp, bruker du gren helt normalt:

```bash
# Start TUI med navigasjon
gren

# CLI navigasjon
gren navigate worktree-navn
gcd worktree-navn           # Kort alias
gnav worktree-navn          # Alternativ alias

# Alle vanlige gren-kommandoer fungerer som før
gren list
gren create -n ny-feature
gren delete gammel-feature
```

## TUI Navigation

1. Kjør `gren`
2. Bruk piltaster for å velge worktree
3. Trykk `g` for å navigere til worktree
4. TUI avsluttes og du er i worktree-mappen

## CLI Navigation

```bash
# Se tilgjengelige worktrees
gren list

# Naviger til en worktree
gcd feature-branch

# Du er nå i worktree-mappen
pwd  # Viser path til worktree
```

## Feilsøking

### "Navigation fungerer ikke"
Shell integration er ikke satt opp riktig:
```bash
# Sjekk at gren er installert
gren --version

# Generer shell integration på nytt
gren shell-init zsh   # eller bash/fish

# Legg til i shell-profil og reload
```

### Navigasjon fungerer ikke
```bash
# Sjekk at wrapper-funksjonen er lastet
type gren

# Test manuelt
gren navigate test-worktree
cat /tmp/gren_navigate
eval "$(cat /tmp/gren_navigate)"
```

### TUI viser ikke 'g' key binding
Du bruker gammel versjon av gren:
```bash
brew update
brew upgrade gren
```

## Migration fra lokal installasjon

Hvis du tidligere brukte lokale wrapper-scripts:

1. Fjern gamle scripts fra shell-profil
2. Installer via Homebrew
3. Sett opp shell integration som beskrevet over

```bash
# Fjern gamle linjer fra ~/.zshrc
# source /path/to/gren-nav.sh
# gren-nav() { ... }

# Legg til ny linje
eval "$(gren shell-init zsh)"
```

## Fordeler med Homebrew-versjonen

- ✅ Automatiske oppdateringer via `brew upgrade`
- ✅ Konsistent installasjon på tvers av maskiner
- ✅ Ingen manuelle script-filer å vedlikeholde
- ✅ Built-in shell integration
- ✅ Support for alle hovedshells (bash, zsh, fish)
- ✅ Clean uninstall via `brew uninstall gren`

## Avinstallasjon

```bash
# Fjern gren
brew uninstall gren

# Fjern shell integration (valgfritt)
# Fjern eller kommenter ut denne linjen fra shell-profil:
# eval "$(gren shell-init zsh)"
```

Shell integration linjen vil ikke forårsake feil selv om gren er avinstallert, men du kan fjerne den for å holde shell-profilen ren.