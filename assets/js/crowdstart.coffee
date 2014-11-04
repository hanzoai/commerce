app = (require './app')
  cookieName: 'SKULLYSystemsCart'

cart     = require './routes/cart'
products = require './routes/products'

app.routes =
  '/cart':          cart.hideHover
  '/products/*':    products.gallery
  '/products/ar-1': products.customizeAr1
  '*':              [cart.click, cart.updateHover]

app.start()
