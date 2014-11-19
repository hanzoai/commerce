View         = require 'mvstar/lib/view'
LineItemView = require './line-item'
util         = require '../util'

cart = app.get 'cart'

class CartView extends View
  el: '.sqs-fullpage-shopping-cart-content'
  bindings:
    subtotal: '.subtotal .price span'

  formatters:
    subtotal: (v) -> util.formatCurrency v

  render: ->
    $('.cart-container tbody').html ''
    index = 0

    @set 'quantity', cart.quantity
    @set 'subtotal', cart.subtotal

    for sku, item of cart.items()
      item.index = ++index
      view = new LineItemView state: item
      window.view = view
      view.render()
      view.bind()
      $('.cart-container tbody').append view.$el

    cart.on 'subtotal', (subtotal) =>
      @set 'subtotal', subtotal

module.exports = CartView
