View = require 'mvstar/lib/view'
util = require '../util'

cart = app.get 'cart'

class CartHover extends View
  el: '.fixed-cart'

  bindings:
    quantity: '.total-quantity'
    subtotal: '.subtotal .price span'
    suffix:   '.details span.suffix'

  formatters:
    quantity: (v) -> util.humanizeNumber v
    subtotal: (v) -> util.formatCurrency v
    suffix:   (v) -> if v > 1 then 'items' else 'item'

  listen: ->
    # listen to cart changes
    cart.on 'quantity', (quantity) =>
      @set 'quantity', quantity
      @set 'suffix', quantity

    cart.on 'subtotal', (subtotal) =>
      @set 'subtotal', subtotal

    # set initial values
    @set 'quantity', cart.quantity
    @set 'suffix',   cart.quantity
    @set 'subtotal', cart.subtotal

module.exports = CartHover
