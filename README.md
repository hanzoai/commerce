# crowdstart
Crowdfunding platform.

## Development
Setup your local development enviroment, installing the deps and the SDK and creating
symlink from `src/` to `sdk/gopath`.

```bash
$ make deps
```

Optionally you can install the normal go cli tools into your local `sdk`:

```bash
$ make tools
```

You can `make serve` to run development server or `make test` to run tests.

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
Part of `api` module. See [`api/models`](blob/master/api/models/models.go) package.
