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

### Installing dependencies
We use Go vendoring and the [govendor](https://github.com/kardianos/govendor)
tool to manage deps. All go deps should be added to the vendor/vendor.json which
govendor maintains.


Installing a new dependency:
```bash
$ govendor fetch golang.org/x/net/context
```

Changing version of a dependency:
```bash
$ govendor fetch golang.org/x/net/context@a4bbce9fcae005b22ae5443f6af064d80a6f5a55
```

## Semantics
There are a number of high-level semantics that are important to the overall
functioning of the platform.

### Caching and invalidation
A number of entities (and, therefore, URL paths that get called) are
aggressively cached via Cloudflare and are only invalidated when the entities
change.  All publically accessible records which are global to an Organization
should be cached. Customer-unique records are not accessed enough to make
caching valuable. These entities/paths are:

- Product `api.hanzo.io/product`
- Bundle `api.hanzo.io/bundle`
- Variant `api.hanzo.io/variant`
- Coupon `api.hanzo.io/coupon`
- Store `api.hanzo.io/store`
- Form JS snippets `api.hanzo.io/form/*/js`
- Organization JS snippets `api.hanzo.io/organization/*/js`
