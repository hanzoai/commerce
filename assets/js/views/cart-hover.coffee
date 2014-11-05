View = require '../view'
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

  listen: ->
    cart = app.get 'cart'

    # listen to cart changes
    cart.on 'update', (quantity, subtotal) =>
      @set 'quantity', quantity
      if cart.quantity > 1
        @set 'suffix', 'item'
      else
        @set 'suffix', 'items'

      @set 'subtotal', subtotal

    # trigger first change
    cart.update()

module.exports = CartHover
