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

## Requirements

### Squarespace
Slight template modification + minimal JS to add interactivity to squarespace
page.

#### Cart
- Save shopping info in cookie

#### Product page  (https://skully-staging.squarespace.com/store/ar-1)
- Add cart should add item to crowdstart platform

#### Cart hover (.sqs-pill-shopping-cart)
- should appear on all pages when items are in cart
- button goes to show cart / shopping cart view

#### Shopping cart view (https://skully-staging.squarespace.com/commerce/show-cart)
- List items in cart
- Show total
- Checkout drives to secure.crowdstart.io
    - AJAX call, save cart info into our platform

### Crowdstart
Secure checkout process, customer/order storage.

- Save cart id + info into db
- Read cart info on secure.crowdstart.io

#### secure.crowdstart.io
Landing page/checkout

#### AJAX API
- /api
    - /save-cart/<cart-id>
    - /get-cart/<cart-id>
