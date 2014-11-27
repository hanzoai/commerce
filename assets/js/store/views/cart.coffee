View         = require 'mvstar/lib/view'
LineItemView = require './line-item'
util         = require '../util'

class CartView extends View
  el: '.cart-hover'
  bindings:
    subtotal: '.subtotal .price .money'

  formatters:
    subtotal: (v) -> util.formatCurrency v

  render: ->
    $('.cart-content tbody').html ''
    index = 0

    cart = app.get 'cart'

    @set 'quantity', cart.get 'quantity'
    @set 'subtotal', cart.get 'subtotal'

    for sku, item of cart.getProducts()
      item.index = ++index
      view = new LineItemView state: item
      window.view = view
      view.render()
      view.bind()
      $('.cart-content tbody').append view.$el

    cart.on 'subtotal', (v) =>
      @set 'subtotal', v

module.exports = CartView
