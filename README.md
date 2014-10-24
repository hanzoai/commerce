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
