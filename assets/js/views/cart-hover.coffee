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
    # listen to cart changes
    app.cart.on 'quantity', (e) =>
      console.log 'quantity', e
      @set 'quantity', app.cart.quantity
      if app.cart.quantity > 1
        @set 'suffix', 'item'
      else
        @set 'suffix', 'items'

    app.cart.on 'subtotal', (e) =>
      console.log 'subtotal', e
      @set 'subtotal', app.cart.subtotal

    # trigger first change
    cart.emit 'quantity', cart.quantity
    cart.emit 'subtotal', cart.quantity

module.exports = CartHover
