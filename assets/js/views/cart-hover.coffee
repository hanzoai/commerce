View = require '../view'
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

  listen: ->
    # listen to cart changes
    cart.on 'update', (quantity, subtotal) =>
      @update quantity, subtotal

    # manually trigger first update
    @update cart.quantity, cart.subtotal

  update: (quantity, subtotal) ->
    @set 'quantity', quantity
    @set 'subtotal', subtotal

    if quantity > 1
      @set 'suffix', 'item'
    else
      @set 'suffix', 'items'


module.exports = CartHover
