View = require 'mvstar/lib/view'
util = require '../util'

class CartHover extends View
  el: '.cart-hover'

  bindings:
    quantity: '.cart-total-quantity'
    subtotal: '.cart-price span'
    suffix:   '.cart-suffix'

  formatters:
    subtotal: (v) -> util.formatCurrency v
    suffix:   (v) -> if v > 1 then 'items' else 'item'
    quantity: (v) ->
      @showCart() if v > 0
      util.humanizeNumber v

  showCart: ->
    @el.animate {opacity: 1}, 400, 'swing'

  listen: ->
    cart = app.get 'cart'

    # listen to cart changes
    cart.on 'quantity', (v) =>
      @set 'quantity', v
      @set 'suffix', v

    cart.on 'subtotal', (v) =>
      @set 'subtotal', v

    # set initial values
    quantity = cart.get 'quantity'
    @set 'quantity', quantity
    @set 'suffix',   quantity
    @set 'subtotal', cart.get 'subtotal'

module.exports = CartHover
