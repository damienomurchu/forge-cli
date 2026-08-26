# Installation

## Supported platforms

Forge supports these release targets:

- Linux on AMD64
- macOS on Apple silicon (ARM64)

The test suite and race detector run natively on both target platforms in CI.

Forge does not currently support Windows. Other operating systems and architectures
may compile but are not release targets and are not supported.

Forge is distributed as a native, CGO-free executable and does not require a
separate SQLite installation. Interactive capture requires a terminal. Bash, Fish,
or Zsh is required only when using the corresponding completion script.

## Release archive

Once published, download the archive for your platform and its checksum from the
[GitHub Releases page](https://github.com/damienomurchu/forge-cli/releases). Verify
the archive against the published SHA-256 checksums before extracting it.

Place the `forge` executable in a directory on `PATH`, for example:

```sh
install -m 0755 forge "$HOME/.local/bin/forge"
forge --version
```

Ensure `$HOME/.local/bin` is on `PATH` if you use that location.

## Build from source

Install the Go version pinned in `mise.toml`, then clone the repository and build:

```sh
git clone https://github.com/damienomurchu/forge-cli.git
cd forge-cli
go build -trimpath -o bin/forge ./cmd/forge
install -m 0755 bin/forge "$HOME/.local/bin/forge"
```

Source builds report version `dev` unless a version is injected at build time.

## Shell completion

### Bash

```sh
mkdir -p "$HOME/.local/share/bash-completion/completions"
forge completion bash > "$HOME/.local/share/bash-completion/completions/forge"
```

This location requires the `bash-completion` package to be installed and loaded.

### Fish

```fish
mkdir -p ~/.config/fish/completions
forge completion fish > ~/.config/fish/completions/forge.fish
```

### Zsh

```zsh
mkdir -p ~/.zfunc
forge completion zsh > ~/.zfunc/_forge
```

Add the directory to `fpath` before initializing completion in `.zshrc`:

```zsh
fpath=(~/.zfunc $fpath)
autoload -Uz compinit
compinit
```

Start a new shell after installation, or reload its completion configuration.

## Data location

Forge stores its SQLite database at:

```text
Linux: $XDG_DATA_HOME/forge/forge.db
       falling back to ~/.local/share/forge/forge.db
macOS: ~/Library/Application Support/forge/forge.db
```

Set `FORGE_DATA_DIR` to an absolute directory to override the default. Forge creates
the directory and database lazily on the first confirmed capture.
