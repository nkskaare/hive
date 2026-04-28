# Contributing to Hive

Thanks for your interest! Hive is a small project and contributions are welcome.

## Getting started

```bash
git clone https://github.com/nkskaare/hive.git
cd hive
go build -o hive .
```

Requirements: Go 1.24+, Docker, Git.

## Making changes

1. Fork the repo and create a branch from `main`.
2. Make your changes — keep them focused.
3. Test manually (`hive spawn`, `hive ls`, `hive kill`).
4. Open a pull request with a clear description of what and why.

## What to work on

Check the [issues](https://github.com/nkskaare/hive/issues) for bugs and feature requests. Issues labeled **good first issue** are a great starting point.

If you have an idea that isn't filed yet, open an issue first so we can discuss it before you put in the work.

## Code style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Keep things simple — hive is intentionally a thin wrapper.
- Don't add dependencies unless truly necessary.

## Reporting bugs

Open an issue with:
- What you expected to happen.
- What actually happened.
- Your OS, Go version, and Docker version.
- The relevant part of your `hive.toml` (if applicable).
