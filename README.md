# crowdstart
Crowdfunding platform.

## Requirements

### Squarespace

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
- Save cart id + info into db
- Read cart info on secure.crowdstart.io

#### secure.crowdstart.io
Landing page/checkout

#### AJAX API
- /api
    - /save-cart/<cart-id>
    - /get-cart/<cart-id>
