view = require '../view'
cart = require '../cart'

class CartHover extends View
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

class Cart extends View

class LineItem extends View
  el: '.line-item'

# renderLineItem = (lineItem, index) ->
#   el = templateEl.clone(false)
#   $quantity = el.find(".quantity input")

#   # get list of variants
#   variantInfo = []
#   variantInfo.push lineItem.color  if lineItem.color isnt ""
#   variantInfo.push lineItem.size  if lineItem.size isnt ""
#   el.find("img.thumbnail").attr "src", lineItem.img
#   el.find("input.slug").val(lineItem.slug).attr "name", "Order.Items." + index + ".Product.Slug"
#   el.find("input.sku").val(lineItem.sku).attr "name", "Order.Items." + index + ".Variant.SKU"
#   el.find("a.title").text lineItem.name
#   el.find("div.variant-info").text variantInfo.join(" / ")
#   el.find(".price span").text formatCurrency(lineItem.price)
#   $quantity.val(lineItem.quantity).attr "name", "Order.Items." + index + ".Quantity"

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

# exports.renderCart = (modifiedCart) ->
#   cart = modifiedCart or csio.getCart()
#   numItems = 0
#   subtotal = 0
#   i = 0
#   $(".cart-container tbody").html ""
#   for k of cart
#     lineItem = cart[k]
#     numItems += lineItem.quantity
#     subtotal += lineItem.price * lineItem.quantity
#     csio.renderLineItem lineItem, i
#     i += 1
#   if i is 0
#     $(".cart-container").hide()
#     $(".empty-message").show()
#   else
#     csio.updateSubtotal subtotal
#   return

# $("input,select").keypress (e) ->
#   e.keyCode isnt 13
