# Installation

watchd is a single binary with no runtime dependencies. You don't need Lua installed,
because it is built in.

**Supported systems:** Linux and macOS, on Intel (amd64) and ARM (arm64).

::: tip Beta software
watchd is in [beta](/guide/introduction#project-status). Before upgrading, check the
[release notes](https://github.com/omar0ali/watchd/releases) for **Breaking changes**.
:::

## Download a prebuilt binary

Every [release](https://github.com/omar0ali/watchd/releases) has archives for each system.
Pick yours:

::: code-group

```sh [Linux (amd64)]
VERSION=$(curl -fsSL https://api.github.com/repos/omar0ali/watchd/releases/latest | grep -m1 '"tag_name"' | cut -d'"' -f4)
curl -fsSLO "https://github.com/omar0ali/watchd/releases/download/${VERSION}/watchd_${VERSION#v}_linux_amd64.tar.gz"
tar xzf "watchd_${VERSION#v}_linux_amd64.tar.gz" watchd
install -m 755 watchd ~/.local/bin/
```

```sh [Linux (arm64)]
VERSION=$(curl -fsSL https://api.github.com/repos/omar0ali/watchd/releases/latest | grep -m1 '"tag_name"' | cut -d'"' -f4)
curl -fsSLO "https://github.com/omar0ali/watchd/releases/download/${VERSION}/watchd_${VERSION#v}_linux_arm64.tar.gz"
tar xzf "watchd_${VERSION#v}_linux_arm64.tar.gz" watchd
install -m 755 watchd ~/.local/bin/
```

```sh [macOS (Apple Silicon)]
VERSION=$(curl -fsSL https://api.github.com/repos/omar0ali/watchd/releases/latest | grep -m1 '"tag_name"' | cut -d'"' -f4)
curl -fsSLO "https://github.com/omar0ali/watchd/releases/download/${VERSION}/watchd_${VERSION#v}_darwin_arm64.tar.gz"
tar xzf "watchd_${VERSION#v}_darwin_arm64.tar.gz" watchd
sudo install -m 755 watchd /usr/local/bin/
```

```sh [macOS (Intel)]
VERSION=$(curl -fsSL https://api.github.com/repos/omar0ali/watchd/releases/latest | grep -m1 '"tag_name"' | cut -d'"' -f4)
curl -fsSLO "https://github.com/omar0ali/watchd/releases/download/${VERSION}/watchd_${VERSION#v}_darwin_amd64.tar.gz"
tar xzf "watchd_${VERSION#v}_darwin_amd64.tar.gz" watchd
sudo install -m 755 watchd /usr/local/bin/
```

:::

::: details Verify the download
Each release includes a `checksums.txt`. Download it next to the archive and run:

```sh
curl -fsSLO "https://github.com/omar0ali/watchd/releases/download/${VERSION}/checksums.txt"
sha256sum -c --ignore-missing checksums.txt     # macOS: shasum -a 256 -c --ignore-missing checksums.txt
```
:::

::: tip macOS: "cannot be opened because the developer cannot be verified"
Binaries downloaded with a browser get a quarantine flag. Remove it with
`xattr -d com.apple.quarantine /usr/local/bin/watchd`. Downloads made with `curl`, as above,
aren't affected.
:::

## Install with Go

If you have [Go 1.25 or newer](https://go.dev/dl/):

```sh
go install github.com/omar0ali/watchd@latest
```

The binary goes to `$(go env GOPATH)/bin`, usually `~/go/bin`.

## Build from source

```sh
git clone https://github.com/omar0ali/watchd.git
cd watchd
scripts/run.sh install       # go install, with the version number stamped in
```

Or build the binary and put it wherever you like:

```sh
scripts/run.sh build         # creates ./build/watchd
install -m 755 build/watchd ~/.local/bin/
```

## Put it on your PATH

If `watchd` is "command not found" after installing, add the directory you installed it to
to your `PATH` in `~/.bashrc`, `~/.zshrc` or your shell's equivalent:

```sh
export PATH="$PATH:$HOME/.local/bin"          # prebuilt binary
export PATH="$PATH:$(go env GOPATH)/bin"      # go install
```

## Check that it works

```console
$ watchd version
watchd v0.2.0

$ watchd info
cli:        watchd v0.2.0
socket:     /run/user/1000/watchd/watchd.sock
daemon log: /home/you/.local/state/watchd/daemon.log
daemon:     not running
```

The daemon shows "not running" until you start your first project. That's expected, because
it starts on demand.

## Shell completion

Tab completion covers commands, flags, and the names of your loaded projects and apps.

::: code-group

```sh [bash]
watchd completion bash > ~/.local/share/bash-completion/completions/watchd
```

```sh [zsh]
# make sure the directory is in your $fpath
watchd completion zsh > "${fpath[1]}/_watchd"
```

```sh [fish]
watchd completion fish > ~/.config/fish/completions/watchd.fish
```

:::

Open a new shell for completion to take effect.

## Updating

Install the new version the same way you installed the old one, then restart the daemon so
it runs the new binary:

```sh
watchd off && watchd on
```

`watchd off` stops every project. Run `watchd up` in each one again afterwards.

## Uninstalling

```sh
watchd off                          # stop everything first
rm "$(command -v watchd)"
rm -rf ~/.local/state/watchd        # the daemon log
```
