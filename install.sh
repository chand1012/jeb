#!/bin/sh
set -eu

repo=chand1012/jeb

fail() {
    printf 'jeb install: %s\n' "$*" >&2
    exit 1
}

case "${1:-}" in
    -h|--help)
        cat <<'EOF'
Install the latest stable Jeb release for macOS or Linux.

Usage: sh install.sh

Set JEB_INSTALL_DIR to choose an installation directory.
Default: $HOME/.local/bin
EOF
        exit 0
        ;;
    '') ;;
    *) fail "unexpected argument: $1" ;;
esac

for command_name in curl uname mktemp awk grep tar install mkdir; do
    command -v "$command_name" >/dev/null 2>&1 || fail "$command_name is required"
done

if command -v sha256sum >/dev/null 2>&1; then
    checksum_command=sha256sum
elif command -v shasum >/dev/null 2>&1; then
    checksum_command=shasum
else
    fail 'sha256sum or shasum is required'
fi

case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail 'only Linux and macOS are supported by this installer' ;;
esac

case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) fail "unsupported CPU architecture: $(uname -m)" ;;
esac

latest_url="https://github.com/$repo/releases/latest"
release_url=$(curl --proto '=https' --proto-redir '=https' -fsSL \
    -o /dev/null -w '%{url_effective}' "$latest_url") ||
    fail "no published stable release is available at $latest_url"

case "$release_url" in
    "https://github.com/$repo/releases/tag/"*)
        tag=${release_url##*/}
        ;;
    *) fail "unexpected latest release URL: $release_url" ;;
esac

case "$tag" in
    v[0-9]*) ;;
    *) fail "unexpected release tag: $tag" ;;
esac

archive="jeb_${tag}_${os}_${arch}.tar.gz"
base_url="https://github.com/$repo/releases/download/$tag"
temporary_dir=$(mktemp -d) || fail 'could not create a temporary directory'
trap 'rm -rf "$temporary_dir"' EXIT HUP INT TERM

curl --proto '=https' --proto-redir '=https' -fsSL --retry 3 \
    -o "$temporary_dir/$archive" "$base_url/$archive" ||
    fail "could not download $archive from release $tag"
curl --proto '=https' --proto-redir '=https' -fsSL --retry 3 \
    -o "$temporary_dir/checksums.txt" "$base_url/checksums.txt" ||
    fail "could not download checksums.txt from release $tag"

expected=$(awk -v name="$archive" \
    '$2 == name || $2 == "./" name { print $1 }' \
    "$temporary_dir/checksums.txt")
printf '%s\n' "$expected" | grep -Eq '^[0-9a-fA-F]{64}$' ||
    fail "no valid SHA-256 checksum found for $archive"

if [ "$checksum_command" = sha256sum ]; then
    actual=$(sha256sum "$temporary_dir/$archive" | awk '{ print $1 }')
else
    actual=$(shasum -a 256 "$temporary_dir/$archive" | awk '{ print $1 }')
fi
[ "$actual" = "$expected" ] || fail "SHA-256 checksum mismatch for $archive"

entries=$(tar -tzf "$temporary_dir/$archive") || fail "invalid archive: $archive"
[ "$entries" = jeb ] || fail "unexpected archive contents: $archive"
entry_type=$(tar -tvzf "$temporary_dir/$archive" | awk 'NR == 1 { print substr($1, 1, 1) }') ||
    fail "could not inspect $archive"
[ "$entry_type" = - ] || fail "archive does not contain a regular jeb binary"
tar -xzf "$temporary_dir/$archive" -C "$temporary_dir" ||
    fail "could not extract $archive"

if [ -n "${JEB_INSTALL_DIR:-}" ]; then
    install_dir=$JEB_INSTALL_DIR
else
    [ -n "${HOME:-}" ] || fail 'set HOME or JEB_INSTALL_DIR before installing'
    install_dir=$HOME/.local/bin
fi
mkdir -p "$install_dir" || fail "could not create $install_dir"
install -m 0755 "$temporary_dir/jeb" "$install_dir/jeb" ||
    fail "could not install to $install_dir/jeb"

printf 'Installed Jeb %s to %s/jeb\n' "$tag" "$install_dir"
case ":${PATH:-}:" in
    *":$install_dir:"*) ;;
    *) printf 'Add %s to your PATH to run jeb from any directory.\n' "$install_dir" ;;
esac
