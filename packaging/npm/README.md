# @mogesang/nexus-cli

A CLI for governing **Nexus Repository 3.76** guest / anonymous access.

This npm package is a thin wrapper: on install it downloads the prebuilt Go
binary for your platform from GitHub Releases and exposes it as `nexus-cli`.
The source and full documentation live at
https://github.com/mogesangop/nexus-cli.

## Install

```sh
npm i -g @mogesang/nexus-cli
nexus-cli --help
```

Or one-off:

```sh
npx @mogesang/nexus-cli --help
```

Supported platforms: linux / macOS / Windows on x64 / arm64.

## Why an npm package for a Go binary?

So teams that already manage tooling through npm (CI images, devcontainers)
can install `nexus-cli` alongside their other dev tools without a separate
download step or a Homebrew/RPM dependency.
