# crowdstart
Crowdfunding platform.

## Development
Setup your enviroment by running `make deps`. This will download the appengine
sdk to `sdk/` and install any dependencies. A `.env` file will set your
enviromental variables, you can use autoenv to automatically source the file
when you enter the directory or manually `source .env` yourself.

The Makefile has a few useful commands, like `serve`, `test`, etc.

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
