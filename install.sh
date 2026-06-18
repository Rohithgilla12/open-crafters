#!/bin/sh
# open-crafters installer.
#
#   curl -fsSL https://raw.githubusercontent.com/Rohithgilla12/open-crafters/main/install.sh | sh
#
# Downloads a prebuilt grader (no Go required), fetches the challenge content,
# and puts the `crafters` launcher on your PATH. Re-run any time to update.
#
# Honors:
#   CRAFTERS_SRC=/path   use a local checkout instead of cloning from GitHub
#   CRAFTERS_HOME=/path  install location (default: ~/.open-crafters)

set -eu

REPO="Rohithgilla12/open-crafters"
HOME_DIR="${CRAFTERS_HOME:-$HOME/.open-crafters}"
REPO_DIR="$HOME_DIR/repo"
BIN_DIR="$HOME_DIR/bin"

say()  { printf '%s\n' "$*"; }
step() { printf '\033[1m→ %s\033[0m\n' "$*"; }
die()  { printf '\033[31merror: %s\033[0m\n' "$*" >&2; exit 1; }

# --- detect platform ---
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in linux|darwin) ;; *) die "unsupported OS '$os' (need linux or darwin). You can still clone the repo and run ./crafters." ;; esac
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) die "unsupported arch '$arch' (need amd64 or arm64)." ;;
esac

mkdir -p "$BIN_DIR"

# --- 1. challenge content (docs + starters the launcher copies from) ---
step "Fetching challenge content"
if [ -n "${CRAFTERS_SRC:-}" ]; then
  rm -rf "$REPO_DIR"; mkdir -p "$REPO_DIR"
  ( cd "$CRAFTERS_SRC" && tar cf - challenges tester crafters docs README.md 2>/dev/null ) | ( cd "$REPO_DIR" && tar xf - )
  say "  copied from $CRAFTERS_SRC"
elif command -v git >/dev/null 2>&1; then
  if [ -d "$REPO_DIR/.git" ]; then
    git -C "$REPO_DIR" pull --ff-only --quiet && say "  updated $REPO_DIR"
  else
    rm -rf "$REPO_DIR"
    git clone --depth 1 --quiet "https://github.com/$REPO.git" "$REPO_DIR" && say "  cloned into $REPO_DIR"
  fi
else
  step "git not found — downloading a tarball instead"
  tmp="$(mktemp -d)"
  curl -fsSL "https://codeload.github.com/$REPO/tar.gz/refs/heads/main" -o "$tmp/oc.tgz" || die "could not download content tarball"
  rm -rf "$REPO_DIR"; mkdir -p "$REPO_DIR"
  tar xzf "$tmp/oc.tgz" -C "$tmp"
  mv "$tmp"/open-crafters-*/* "$REPO_DIR"/
  rm -rf "$tmp"
  say "  extracted into $REPO_DIR"
fi

# --- 2. the grader binary (prefer prebuilt, fall back to building) ---
asset="crafters-tester_${os}_${arch}"
url="https://github.com/$REPO/releases/latest/download/$asset"
step "Installing the grader ($os/$arch)"
if curl -fsSL "$url" -o "$BIN_DIR/crafters-tester" 2>/dev/null; then
  chmod +x "$BIN_DIR/crafters-tester"
  say "  downloaded prebuilt grader (no Go needed)"
elif command -v go >/dev/null 2>&1; then
  say "  no prebuilt release for $os/$arch yet — building from source with Go"
  ( cd "$REPO_DIR/tester" && go build -o "$BIN_DIR/crafters-tester" ./cmd/tester ) || die "go build failed"
  say "  built $BIN_DIR/crafters-tester"
else
  die "no prebuilt grader for $os/$arch and Go isn't installed. Install Go (https://go.dev/dl) and re-run this script."
fi

# --- 3. the launcher ---
step "Installing the crafters launcher"
cp "$REPO_DIR/crafters" "$BIN_DIR/crafters"
chmod +x "$BIN_DIR/crafters"
say "  installed $BIN_DIR/crafters"

# --- 4. PATH ---
profile=""
case "${SHELL:-}" in
  *zsh)  profile="$HOME/.zshrc" ;;
  *bash) [ "$os" = "darwin" ] && profile="$HOME/.bash_profile" || profile="$HOME/.bashrc" ;;
  *)     profile="$HOME/.profile" ;;
esac
line="export PATH=\"$BIN_DIR:\$PATH\""
on_path=0
case ":$PATH:" in *":$BIN_DIR:"*) on_path=1 ;; esac

say ""
if [ "$on_path" -eq 1 ]; then
  step "Done. 'crafters' is ready."
elif [ -n "$profile" ] && ! { [ -f "$profile" ] && grep -qF "$BIN_DIR" "$profile"; }; then
  printf '\n# open-crafters\n%s\n' "$line" >> "$profile"
  step "Done. Added $BIN_DIR to PATH in $profile"
  say "  Open a new terminal, or run:  $line"
else
  step "Done. Add this to your shell profile, then restart your terminal:"
  say "  $line"
fi

say ""
say "Get started:"
say "  crafters list               # see all challenges"
say "  crafters start wal          # scaffold the WAL challenge and grade it"
say "  cd my-wal && crafters test  # after editing your code"
