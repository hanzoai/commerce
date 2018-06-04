# Hanzo [![Build status](https://badge.buildkite.com/d7e68217b7c11a402384e82726433b30b9ebdb54cab934d89c.svg)](https://buildkite.com/hanzo/platform)

Hanzo is a modern blockchain development platform.

## Development

### Getting started
You can use `make` to setup your development enviroment. Running:

```
$ make deps
```

...will download the Go App Engine SDK and unzip it into `sdk/`. When hacking on
things you'll want to ensure `$GOROOT` and `$GOPATH` point to their respective
directories inside `sdk/`.

You can source the provided `.env` file to set these variables, or use
[`autoenv`](https://github.com/kennethreitz/autoenv) to set them automatically
whenever you `cd` into the project directory.

### Installing Go tools
You can install the common Go command line tools and configure `gocode` to work
with App Engine by running:

```bash
$ make tools
```

### Development server
You can then use `make serve` to run the local development server and `make
test` to run tests.

You can create a local `config.json` file containing configuration variables to
customize settings locally (for instance to disable the auto fixture loading).
