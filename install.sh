#!/bin/sh
# open-crafters installer.
#
#   curl -fsSL https://raw.githubusercontent.com/Rohithgilla12/open-crafters/main/install.sh | sh
#
# Downloads the single self-contained `crafters` binary (challenge content is
# embedded — no repo checkout, no Go required) and puts it on your PATH.
# Re-run any time to update.
#
# Honors CRAFTERS_HOME (install location, default ~/.open-crafters).

set -eu

REPO="Rohithgilla12/open-crafters"
BIN_DIR="${CRAFTERS_HOME:-$HOME/.open-crafters}/bin"

step() { printf '\033[1m→ %s\033[0m\n' "$*"; }
die()  { printf '\033[31merror: %s\033[0m\n' "$*" >&2; exit 1; }

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in linux|darwin) ;; *) die "unsupported OS '$os' (need linux or darwin)." ;; esac
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) die "unsupported arch '$arch' (need amd64 or arm64)." ;;
esac

mkdir -p "$BIN_DIR"
asset="crafters_${os}_${arch}"
url="https://github.com/$REPO/releases/latest/download/$asset"

step "Installing crafters ($os/$arch)"
if curl -fsSL "$url" -o "$BIN_DIR/crafters" 2>/dev/null; then
  chmod +x "$BIN_DIR/crafters"
  printf '  downloaded prebuilt binary\n'
elif command -v go >/dev/null 2>&1; then
  printf "  no prebuilt release for %s/%s yet — building with 'go install'\n" "$os" "$arch"
  GOBIN="$BIN_DIR" go install "github.com/$REPO/cmd/crafters@latest" || die "go install failed"
else
  die "no prebuilt binary for $os/$arch and Go isn't installed. Install Go (https://go.dev/dl) and re-run."
fi

# --- PATH ---
profile=""
case "${SHELL:-}" in
  *zsh)  profile="$HOME/.zshrc" ;;
  *bash) [ "$os" = "darwin" ] && profile="$HOME/.bash_profile" || profile="$HOME/.bashrc" ;;
  *)     profile="$HOME/.profile" ;;
esac
line="export PATH=\"$BIN_DIR:\$PATH\""
on_path=0
case ":$PATH:" in *":$BIN_DIR:"*) on_path=1 ;; esac

printf '\n'
if [ "$on_path" -eq 1 ]; then
  step "Done. 'crafters' is ready."
elif [ -n "$profile" ] && ! { [ -f "$profile" ] && grep -qF "$BIN_DIR" "$profile"; }; then
  printf '\n# open-crafters\n%s\n' "$line" >> "$profile"
  step "Done. Added $BIN_DIR to PATH in $profile"
  printf '  Open a new terminal, or run:  %s\n' "$line"
else
  step "Done. Add this to your shell profile, then restart your terminal:"
  printf '  %s\n' "$line"
fi

printf '\nGet started:\n'
printf '  crafters                    # interactive dashboard\n'
printf '  crafters start wal          # scaffold the WAL challenge and grade it\n'
printf '  cd my-wal && crafters test  # after editing your code\n'
