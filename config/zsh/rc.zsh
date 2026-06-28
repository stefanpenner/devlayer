# Shared zsh config — managed by devlayer
# Sourced from ~/.zshrc before machine-specific config.

# --- Helpers ---

# Prepend directories to PATH (skips missing dirs, deduplicates)
path_prepend() {
  for dir in "$@"; do
    [[ -d "$dir" ]] && path=("$dir" $path)
  done
  typeset -U path
}

# --- Devlayer tools ---

path_prepend "$HOME/.local/bin"

# --- Accessibility wrappers ---

copilot() {
  command copilot --screen-reader --no-mouse --plain-diff "$@"
}

# --- Zsh plugins (installed by devlayer to ~/.local/share) ---

local _share="$HOME/.local/share"

# Powerlevel10k theme
[[ -f "$_share/powerlevel10k/powerlevel10k.zsh-theme" ]] && \
  source "$_share/powerlevel10k/powerlevel10k.zsh-theme"

# Autosuggestions
[[ -f "$_share/zsh-autosuggestions/zsh-autosuggestions.zsh" ]] && \
  source "$_share/zsh-autosuggestions/zsh-autosuggestions.zsh"

# Syntax highlighting
[[ -f "$_share/zsh-fast-syntax-highlighting/fast-syntax-highlighting.plugin.zsh" ]] && \
  source "$_share/zsh-fast-syntax-highlighting/fast-syntax-highlighting.plugin.zsh"

# History substring search
[[ -f "$_share/zsh-history-substring-search/zsh-history-substring-search.zsh" ]] && \
  source "$_share/zsh-history-substring-search/zsh-history-substring-search.zsh"

# fzf key-bindings & completion
[[ -f "$_share/fzf/key-bindings.zsh" ]] && source "$_share/fzf/key-bindings.zsh"
[[ -f "$_share/fzf/completion.zsh" ]]   && source "$_share/fzf/completion.zsh"

# --- p10k config ---
[[ -f ~/.p10k.zsh ]] && source ~/.p10k.zsh
