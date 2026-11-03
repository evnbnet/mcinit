#!/bin/sh
# mcinit installer for macOS
#   curl -fsSL https://github.com/evnbnet/mcinit/releases/latest/download/install.sh | sh

set -eu

REPO="evnbnet/mcinit"
BIN_DIR="${MCINIT_BIN_DIR:-$HOME/.local/bin}"

if [ -t 1 ]; then
  ACCENT=$(printf '\033[38;2;125;86;244m')
  DIM=$(printf '\033[90m')
  GREEN=$(printf '\033[38;2;4;181;117m')
  DANGER=$(printf '\033[38;2;255;95;135m')
  RESET=$(printf '\033[0m')
else
  ACCENT='' DIM='' GREEN='' DANGER='' RESET=''
fi

step() { printf '  %s+%s %s\n' "$GREEN" "$RESET" "$1"; }
note() { printf '    %s%s%s\n' "$DIM" "$1" "$RESET"; }
die() { printf '  %sx%s %s\n' "$DANGER" "$RESET" "$1" >&2; exit 1; }

[ "$(uname -s)" = "Darwin" ] || die "This installer is for macOS. On Windows use install.ps1."

case "$(uname -m)" in
  arm64) ARCH="arm64" ;;
  x86_64) ARCH="amd64" ;;
  *) die "Unsupported architecture: $(uname -m)" ;;
esac

for tool in curl tar shasum; do
  command -v "$tool" >/dev/null 2>&1 || die "$tool is required but not installed."
done

printf '\n  %smcinit%s installer\n\n' "$ACCENT" "$RESET"

step 'Looking up the latest release'
TAG=$(curl -fsSL -H 'Accept: application/vnd.github+json' \
  "https://api.github.com/repos/$REPO/releases/latest" |
  sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
[ -n "$TAG" ] || die "Could not work out the latest release of $REPO."

VERSION="${TAG#v}"
ASSET="mcinit_${VERSION}_darwin_${ARCH}.tar.gz"
BASE="https://github.com/$REPO/releases/download/$TAG"
note "$TAG · darwin $ARCH"

TEMP=$(mktemp -d)
trap 'rm -rf "$TEMP"' EXIT INT TERM

step "Downloading $ASSET"
curl -fsSL "$BASE/$ASSET" -o "$TEMP/$ASSET" ||
  die "Could not download $ASSET. Does release $TAG include a macOS $ARCH build?"

if curl -fsSL "$BASE/checksums.txt" -o "$TEMP/checksums.txt" 2>/dev/null; then
  # Matched as a whole field: the asset name contains dots, which grep would
  # treat as wildcards.
  EXPECTED=$(awk -v want="$ASSET" '$2 == want { print $1 }' "$TEMP/checksums.txt" | head -n 1)
  if [ -n "$EXPECTED" ]; then
    ACTUAL=$(shasum -a 256 "$TEMP/$ASSET" | awk '{print $1}')
    [ "$ACTUAL" = "$EXPECTED" ] ||
      die "SHA256 mismatch for $ASSET (expected $EXPECTED, got $ACTUAL)"
    step 'SHA256 verified'
  else
    note "checksums.txt does not mention $ASSET, so the download was not verified"
  fi
else
  note 'This release has no checksums.txt, so the download was not verified'
fi

step "Installing into $BIN_DIR"
mkdir -p "$BIN_DIR"
tar -xzf "$TEMP/$ASSET" -C "$TEMP"
[ -f "$TEMP/mcinit" ] || die 'The archive does not contain mcinit.'
mv -f "$TEMP/mcinit" "$BIN_DIR/mcinit"
chmod +x "$BIN_DIR/mcinit"

# Unsigned binary: without this the first run dies on Gatekeeper.
if command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$BIN_DIR/mcinit" 2>/dev/null || true
fi

printf '\n  %sDone%s %s->%s %smcinit new my-server%s\n\n' \
  "$ACCENT" "$RESET" "$DIM" "$RESET" "$ACCENT" "$RESET"

case ":$PATH:" in
  *":$BIN_DIR:"*)
    ;;
  *)
    note "$BIN_DIR is not on your PATH. Add it with:"
    case "${SHELL##*/}" in
      zsh) note "  echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.zshrc && exec zsh" ;;
      bash) note "  echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.bash_profile && exec bash" ;;
      fish) note "  fish_add_path $BIN_DIR" ;;
      *) note "  export PATH=\"$BIN_DIR:\$PATH\"" ;;
    esac
    printf '\n'
    ;;
esac
