LineItemView = require './line-item'
View = require '../view'
util = require '../util'

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
      item.index = index++
      view = new LineItemView state: item
      window.view = view
      view.render()
      view.bind()
      $('.cart-container tbody').append view.$el

      cart.on 'subtotal', (subtotal) => @set 'subtotal', subtotal

module.exports = CartView

# EVENTSSSS
#   # Handle quantity changes
#   $quantity.change (e) ->
#     e.preventDefault()
#     e.stopPropagation()

#     # Get quantity
#     quantity = parseInt($(this).val(), 10)

#     # Prevent less than one quantity
#     if quantity < 1
#       quantity = 1
#       $(this).val 1

#     # Update quantity
#     lineItem.quantity = quantity

#     # Update line item
#     csio.updateLineItem lineItem, el
#     return


#   # Handle lineItem removals
#   el.find(".remove-item").click ->
#     csio.removeLineItem lineItem.sku, el
#     return

#   el.removeClass "template"
#   $(".cart-container tbody").append el
#   return


# $("input,select").keypress (e) ->
#   e.keyCode isnt 13
