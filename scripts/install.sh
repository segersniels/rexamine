#!/bin/sh
# Installs rexdiff for this machine from the GitHub releases, then runs
# `rexdiff setup` to add it to Rex and install the agent skill.
#
#   curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/segersniels/rexdiff/main/scripts/install.sh | sh
#
# Environment variables:
#   REXDIFF_INSTALL_DIR  Where to put the binary. Defaults to ~/.local/bin.
#   REXDIFF_VERSION      The release to install, such as 0.2.0. Defaults to the latest.
#   REXDIFF_NO_SETUP     Set to 1 to skip `rexdiff setup`.
#
# Everything runs from main at the end, so a partly downloaded script does nothing.
set -eu

REPO="segersniels/rexdiff"

say() {
  printf 'rexdiff: %s\n' "$1"
}

fail() {
  printf 'rexdiff: error: %s\n' "$1" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "this script needs $1"
}

# detect_platform prints the release's name for this OS and CPU, such as darwin_arm64.
detect_platform() {
  case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "rexdiff runs on macOS and Linux, not $(uname -s)" ;;
  esac

  case "$(uname -m)" in
  arm64 | aarch64) arch=arm64 ;;
  x86_64 | amd64) arch=amd64 ;;
  *) fail "rexdiff runs on arm64 and x86_64, not $(uname -m)" ;;
  esac

  # A shell under Rosetta reports x86_64 on an Apple silicon Mac. The native binary is faster.
  if [ "$os" = darwin ] && [ "$arch" = amd64 ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null)" = 1 ]; then
    arch=arm64
  fi

  printf '%s_%s' "$os" "$arch"
}

# sha256 prints the SHA-256 checksum of a file.
sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
  else
    shasum -a 256 "$1" | awk '{ print $1 }'
  fi
}

main() {
  need curl
  need tar
  need uname

  install_dir="${REXDIFF_INSTALL_DIR:-$HOME/.local/bin}"
  base_url="https://github.com/$REPO/releases/latest/download"
  if [ -n "${REXDIFF_VERSION:-}" ]; then
    base_url="https://github.com/$REPO/releases/download/v${REXDIFF_VERSION#v}"
  fi

  archive="rexdiff_$(detect_platform).tar.gz"
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  say "downloading $archive"
  curl --proto '=https' --tlsv1.2 -fsSL -o "$tmp_dir/$archive" "$base_url/$archive" ||
    fail "could not download $base_url/$archive"
  curl --proto '=https' --tlsv1.2 -fsSL -o "$tmp_dir/checksums.txt" "$base_url/checksums.txt" ||
    fail "could not download the checksums of the release"

  # Stop when the download does not match the checksum that the release publishes.
  expected="$(awk -v file="$archive" '$2 == file { print $1 }' "$tmp_dir/checksums.txt")"
  [ -n "$expected" ] && [ "$expected" = "$(sha256 "$tmp_dir/$archive")" ] ||
    fail "the checksum of $archive does not match the release"

  tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" rexdiff
  mkdir -p "$install_dir"

  # Move a new file into place: macOS kills a binary that is overwritten in place.
  chmod 755 "$tmp_dir/rexdiff"
  mv "$tmp_dir/rexdiff" "$install_dir/rexdiff"
  say "installed $("$install_dir/rexdiff" --version) to $install_dir/rexdiff"

  if [ "${REXDIFF_NO_SETUP:-}" != 1 ]; then
    echo
    "$install_dir/rexdiff" setup
  fi

  case ":$PATH:" in
  *":$install_dir:"*) ;;
  *)
    echo
    say "$install_dir is not in your PATH. Add it to your shell's configuration:"
    case "$(basename "${SHELL:-sh}")" in
    fish) echo "  fish_add_path $install_dir" ;;
    zsh) echo "  echo 'export PATH=\"$install_dir:\$PATH\"' >> ~/.zshenv" ;;
    bash) echo "  echo 'export PATH=\"$install_dir:\$PATH\"' >> ~/.bashrc" ;;
    *) echo "  export PATH=\"$install_dir:\$PATH\"" ;;
    esac
    ;;
  esac
}

main "$@"
