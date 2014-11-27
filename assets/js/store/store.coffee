App = require 'mvstar/lib/app'
routes = require './routes'

class StoreApp extends App
  prefix: '/:store?'

  routes:
    '/cart': [
      routes.cart.hideHover
      routes.cart.setupView
    ]

    '/products/:slug': [
      routes.cart.setupHover
      routes.products.gallery
      routes.products.setupViews
    ]

    '/products/ar-1': [
      routes.products.customizeAr1
    ]

    '/:prefix?': routes.cart.setupHover

    '/store': [
      routes.cart.setupHover
      routes.products.setupViews
      routes.store.gallery
      routes.store.setupStyles
    ]

    '*': routes.cart.click

  start: ->
    # create cart and fetch state from cookie
    cart = new (require './models/cart')
    cart.fetch()

    # Alert popup
    alert = new (require './views/alert')
      nextTo: '.sqs-add-to-cart-button'

    # store cart/alert so they can be easily accessed from views
    @set 'cart', cart
    @set 'alert', alert

    # trigger route callbacks
    @route()

window.app = app = new StoreApp()

# let us begin
app.start()
