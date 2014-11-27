App = require 'mvstar/lib/app'
routes = require './routes'

class StoreApp extends App
  prefix: '/:store?'

  routes:
    '/': [
      routes.cart.setupHover
      routes.store.setupViews
      routes.store.gallery
      routes.store.setupStylesAndSizes
    ]

    '/cart': [
      routes.cart.hideHover
      routes.cart.setupViews
    ]

    '/:prefix?': routes.cart.setupHover

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
$(document).ready -> app.start()
