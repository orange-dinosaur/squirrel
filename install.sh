#!/bin/sh
# Installs squirrel from its GitHub Releases:
#   curl -fsSL https://raw.githubusercontent.com/orange-dinosaur/squirrel/main/install.sh | sh
# SQUIRREL_VERSION pins a Release (0.1.0 or v0.1.0). SQUIRREL_INSTALL_DIR changes the folder
# (default ~/.local/bin).
set -eu

releases=https://github.com/orange-dinosaur/squirrel/releases

fail() {
	echo "squirrel: $*" >&2
	exit 1
}

main() {
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	case $(uname -m) in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) arch=$(uname -m) ;;
	esac
	case $os/$arch in
	darwin/amd64 | darwin/arm64 | linux/amd64 | linux/arm64) ;;
	*) fail "no release for $os/$arch" ;;
	esac

	if [ -n "${SQUIRREL_VERSION:-}" ]; then
		tag=v${SQUIRREL_VERSION#v}
	else
		url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$releases/latest") ||
			fail "can't reach GitHub"
		case $url in
		*/releases/tag/*) tag=${url##*/} ;;
		*) fail "no release published yet" ;;
		esac
	fi
	version=${tag#v}
	asset=squirrel_${version}_${os}_${arch}.tar.gz

	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	curl -fsL -o "$tmp/checksums.txt" "$releases/download/$tag/checksums.txt" ||
		fail "no release $tag"
	want=$(awk -v a="$asset" '$2 == a { print $1 }' "$tmp/checksums.txt")
	[ -n "$want" ] || fail "no release for $os/$arch"
	curl -fsSL -o "$tmp/$asset" "$releases/download/$tag/$asset" || fail "can't download $asset"

	if command -v sha256sum >/dev/null 2>&1; then
		got=$(sha256sum "$tmp/$asset" | cut -d ' ' -f 1)
	elif command -v shasum >/dev/null 2>&1; then
		got=$(shasum -a 256 "$tmp/$asset" | cut -d ' ' -f 1)
	else
		fail "can't check the download: neither sha256sum nor shasum is installed"
	fi
	[ "$got" = "$want" ] || fail "$asset does not match its checksum"

	tar -xzf "$tmp/$asset" -C "$tmp" squirrel
	dir=${SQUIRREL_INSTALL_DIR:-$HOME/.local/bin}
	mkdir -p "$dir"
	chmod 755 "$tmp/squirrel"
	mv -f "$tmp/squirrel" "$dir/squirrel"
	echo "squirrel $version installed to $dir/squirrel"

	case :$PATH: in
	*:"$dir":*) ;;
	*)
		echo
		echo "$dir is not on your PATH. Add this line to your shell's startup file:"
		echo "  export PATH=\"$dir:\$PATH\""
		;;
	esac
}

# Everything runs from main, so a download cut short by curl | sh runs nothing.
main
