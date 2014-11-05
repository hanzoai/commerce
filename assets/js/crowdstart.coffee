window.app = app = (require './app')
  cookieName: 'SKULLYSystemsCart'

# Store cart for later
app.set 'cart', (require './cart')
app.set 'alert' new AlertView nextTo: '.sqs-add-to-cart-button'

cart     = require './routes/cart'
products = require './routes/products'

app.routes =
  '/cart':          [cart.hideHover, cart.setupView]
  '/products/*':    [products.setupView, products.gallery, cart.setupHover]
  '/products/ar-1': [products.customizeAr1]
  '*':              cart.click

app.start()
