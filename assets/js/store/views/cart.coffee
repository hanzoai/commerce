View         = require 'mvstar/lib/view'
LineItemView = require './line-item'
util         = require '../util'

class CartView extends View
  el: '.shopping-cart'
  bindings:
    subtotal: '.subtotal .money'

  formatters:
    subtotal: (v) -> util.formatCurrency v

  render: ->
    $('.cart-content tbody').html ''
    index = 0

    cart = app.get 'cart'

    cart.on 'subtotal', (v) =>
      @set 'subtotal', v

    cart.on 'quantity', (v) =>
      if v == 0
        @el.find('.cart-content').animate {opacity: 0, height: '0px'}, 500
        @el.find('.empty-message').fadeIn(500)

    @set 'subtotal', cart.get 'subtotal'

    for sku, item of cart.getProducts()
      item.index = ++index
      view = new LineItemView state: item
      window.view = view
      view.render()
      view.bind()
      $('.cart-content tbody').append view.$el

    if cart.get('quantity') == 0
      @el.find('.cart-content').hide()
      @el.find('.empty-message').show()

module.exports = CartView
