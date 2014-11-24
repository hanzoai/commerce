View = require 'mvstar/lib/view'
util = require '../util'

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
    cart = app.get 'cart'

    # listen to cart changes
    cart.on 'quantity', (quantity) =>
      @set 'quantity', quantity
      @set 'suffix', quantity

    cart.on 'subtotal', (subtotal) =>
      @set 'subtotal', subtotal

    # set initial values
    @set 'quantity', cart.get 'quantity'
    @set 'suffix',   cart.get 'quantity'
    @set 'subtotal', cart.get 'subtotal'

module.exports = CartHover
