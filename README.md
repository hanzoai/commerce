# crowdstart
Crowdfunding platform.

## Development
You can use `make` to setup your development enviroment. Running:

```
$ make deps
```

...will download the Go App Engine SDK and unzip it into `.sdk/`. When hacking
on things you'll want to ensure `$GOROOT` and `$GOPATH` point to their
respective subdirs inside `.sdk/`.

You can use the provided `.env` file (`source .env`), or
[`autoenv`](https://github.com/kennethreitz/autoenv), which autosources `.env`
files on entry.

Optionally you can also install the common Go command line tools into your local
SDK and configure `gocode` to work with:

```bash
$ make tools
```

You can then use `make serve` to run the local development server and `make
test` to run tests.

## Architecture
Crowdstart is broken up into several different [App Engine
Modules](https://cloud.google.com/appengine/docs/go/modules/).

### `api` module
Implements backend models and provides abstraction layer between Google's
Datastore. `api/models` package is shared across project.

### `checkout` module
Implements secure checkout, generally hosted at `https://secure.crowdstart.io`.

### `default` module
Not a Go app module, only contains static file mappings for Google CDN hosting.

### `admin` module
Should eventually implement some sort of reporting/management interface for
clients.

### `store` module
Implements store frontend, product pages, cart view, order management.

We use `dispatch.yaml` to pipe requests to the proper module. During local
development you can use `localhost:8080` and subdir bound to each module, but
it's presumed that in production a different mapping will be used.

#### Common gotchas with `dispatch.yaml`
- Order matters, first matching pattern takes precedence.
- Subdomains are incompatible with the local development server.
- All `goapp` incantations change once you use modules (see our
  [`Makefile`](Makefile)).

## Frontend UI

### store.crowdstart.io (`store module`)
- Need to display hover when something is in cart with link to show cart page.

### / product listing
- product
    - thumbnail, title

### /product/<slug>
- title
- images
- description
- add to cart
- dropdowns
    - color
    - size
- force color/size stuff to be selected

### /show-cart
- products + total
- checkout

### /account
- Show orders
- Account information

### /create-account
### /login
### /logout
### /reset-password

### /orders/<order-id>
- Show order
- Current status
- Tracking info?
- Ability to manage order up until shipped

### secure.crowdstart.io (`checkout` module)
- Requires SSL.

### /checkout/<cartid>
- billing info
- order summary
- shipping options
- continue
- display errors if unable to direct to complete
- save email / password for login?

### /checkout/complete
    - thank you

## Backend API
## api.crowdstart.io (`api` module)
### /api/cart
- create, get, add, remove stuff from a cart

## Admin UI
### admin.crowdstart.io (`admin` module)

### /login
### /logout

## Models
Part of `api` module. See [`api/models`](api/models/models.go) package.
