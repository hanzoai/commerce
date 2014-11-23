App = require 'mvstar/lib/app'

class StoreApp extends App
  start: ->
    super
    $.cookie.json = true

window.app = app = new StoreApp cookieName: 'SKULLYSystemsCart'

routes = require './routes'

# Store cart for later
app.set 'cart',  (require './cart')
app.set 'alert', new (require './views/alert')
  nextTo: '.sqs-add-to-cart-button'

app.routes =
  '/:prefix?/cart': [
    routes.cart.hideHover
    routes.cart.setupView
  ]

  '/:prefix?/products/:slug': [
    routes.cart.setupHover
    routes.products.gallery
    routes.products.setupView
  ]

  '/:prefix?/products/ar-1': [
    routes.products.customizeAr1
  ]

  '/:prefix?': [
    routes.cart.setupHover
  ]

  '*': routes.cart.click

app.start()
