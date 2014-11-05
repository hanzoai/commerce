window.app = app = (require './app')
  cookieName: 'SKULLYSystemsCart'

cart     = require './routes/cart'
products = require './routes/products'

app.routes =
  '/cart':          cart.hideHover
  '/products/':     [products.alert, products.gallery]
  '/products/ar-1': products.customizeAr1
  '*':              [cart.click, cart.updateHover, (-> alert("hi"))]

app.start()
