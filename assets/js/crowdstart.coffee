window.app = app = (require './app')
  cookieName: 'SKULLYSystemsCart'

cart     = require './routes/cart'
products = require './routes/products'

app.routes =
  '/cart':          [cart.hideHover, cart.setupView]
  '/products/*':    [products.setupView, products.gallery, cart.setupHover]
  '/products/ar-1': [products.customizeAr1]
  '*':              cart.click

# Store cart for later
app.set 'cart', (require './cart')

app.start()
