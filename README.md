# crowdstart [ ![Codeship Status for verus-io/crowdstart](https://www.codeship.io/projects/6d197000-4232-0132-a221-0608fd86df6c/status)](https://www.codeship.io/projects/44348)
Crowdfunding platform.

## Development
You can use `make` to setup your development enviroment. Running:

```
$ make deps
```

...will download the Go App Engine SDK and unzip it into `.sdk/`. When hacking
on things you'll want to ensure `$GOROOT` and `$GOPATH` point to their
respective directories inside `.sdk/`.

You can source the provided `.env` file to set these variables, or
[`autoenv`](https://github.com/kennethreitz/autoenv) to set them automatically
when entering the project directory.

You can install the common Go command line tools and configure `gocode` to work
with App Engine by running:

```bash
$ make tools
```

You can then use `make serve` to run the local development server and `make
test` to run tests.

## Architecture
- Go is used for all backend code.
- Datastore is primary database
- Hosted on Google App Engine

Crowdstart is broken up into several different [App Engine
Modules](https://cloud.google.com/appengine/docs/go/modules/), for clarity,
performance, reliability and scalability reasons. They are:

### `default` (`app.yaml`)
Not a Go app module, contains static file mappings for Google CDN hosting.

### `api` (`api/app.yaml`)
Implements backend models and provides abstraction layer between Google's
Datastore. `api/models` package is shared across project.

### `checkout` (`checkout/app.yaml`)
Implements secure checkout, generally hosted at `https://secure.crowdstart.io`.

### `store` (`store/app.yaml`)
Implements store frontend, product pages, cart view, order management.

### `admin` (`admin/app.yaml`)
Should eventually implement some sort of reporting/management interface for
clients.

### `dispatch.yaml`
We use `dispatch.yaml` to pipe requests to the proper module. Unfortunately
configuring modules to route to different hostnames is not supported during
local development, which is why each module's handlers are set to a different
subdirectory.

#### Gotchas
- Order matters, first matching pattern takes precedence.
- Subdomains are incompatible with the local development server.
- Routing to the same url in multiple Go modules is not allowed (at least
  locally).
- All `goapp` incantations change once you use modules (see our
  [`Makefile`](Makefile) for details).
- If you update an `app.yaml`'s url patterns, make sure to update
  `dispatch.yaml` or they will be ignored.
- You have to run `appcfg.py update_dispatch` for dispatch rules to be used in
  production.

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

## TODO
- Support [multitenancy](https://cloud.google.com/appengine/docs/go/multitenancy/#Go_About_multitenancy).
