# squirrel

squirrel manages your configs and dotfiles on macOS and Linux. It moves them into one folder, the
Store (`~/.squirrel`), lists them in a TOML Manifest, and symlinks them back into your home
directory. It syncs the Store between machines through a git Remote. Run `squirrel` with no
arguments in a terminal for a guided menu.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/orange-dinosaur/squirrel/main/install.sh | sh
```

The script installs to `~/.local/bin` without sudo and checks the download against the Release's
checksums. It never edits your shell's startup files. If the folder is not on your PATH, it prints
the line to add. Set `SQUIRREL_INSTALL_DIR` to install somewhere else, or `SQUIRREL_VERSION=0.1.0`
to install a specific Release.

### Manual download

Download the archive for your system from the
[Releases page](https://github.com/orange-dinosaur/squirrel/releases), unpack it, and move
`squirrel` onto your PATH. On macOS, clear the quarantine flag once, or Gatekeeper refuses to run
the unsigned binary:

```sh
xattr -d com.apple.quarantine squirrel
```

## Upgrade

```sh
squirrel upgrade          # the latest Release
squirrel upgrade v0.1.0   # a specific Release, even an older one
```

## Quick start

On your first machine, move a file into the Store, then connect an empty git repository as the
Remote. `remote add` pushes the Store to it.

```sh
squirrel add ~/.zshrc
squirrel remote add git@github.com:you/dotfiles.git
```

On a second machine, `remote add` with the same URL fills the empty Store and links every Entry
into place. After that, `squirrel push` sends changes and `squirrel pull` brings them in.

```sh
squirrel remote add git@github.com:you/dotfiles.git
```

`squirrel status` shows whether each Entry is linked, and `squirrel <command> --help` explains each
command.

## License

MIT. See [LICENSE](LICENSE).
