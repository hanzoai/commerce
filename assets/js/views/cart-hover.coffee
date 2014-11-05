View = require '../view'
cart = require '../cart'
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

  update: ->
    @set 'quantity', cart.quantity
    @set 'subtotal', cart.subtotal

    if cart.quantity > 1
      @set 'suffix', 'item'
    else
      @set 'suffix', 'items'

module.exports = CartHover
