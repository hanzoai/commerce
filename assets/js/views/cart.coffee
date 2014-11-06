View = require '../view'

cart = app.get 'cart'

class CartView extends View

class LineItemView extends View
  template: '#line-item-template'

  bindings:
    img:        'img.thumbnail   @src'
    slug:       'input.slug      @value'
    name:       'a.title'
    desc:       'div.desc'
    price:      '.price span'
    quantity:   '.quantity input @value'

    index:     ['input.sku       @name'
                'input.slug      @name'
                '.quantity input @name']

    skuIndex:   'input.sku @name'
    slugIndex:  'input.slug @name'
    quantIndex: '.quantity input @name'

  computed:
    desc: (color, size) -> [color, size]

  watching:
    desc: ['color', 'size']

  formatters:
    slug: (v) ->
      'Order.Items.' + v + '.Product.Slug'

    index: (v, selector) ->
      switch selector
        when 'input.sku @name'
          "Order.Items.#{v}.Variant.SKU"
        when 'input.slug @name'
          "Order.Items.#{v}.Product.Slug"

    desc: (v) ->
      v.join ' / '

    price: (v) ->
      util.formatCurrency v

# renderLineItem = (lineItem, index) ->
#   el = templateEl.clone(false)

#   # get list of variants
#   variantInfo = []
#   variantInfo.push lineItem.color  if lineItem.color isnt ""
#   variantInfo.push lineItem.size  if lineItem.size isnt ""
#   el.find('input.slug').val(lineItem.slug).attr 'name', 'Order.Items.' + index + '.Product.Slug'
#   el.find('input.sku').val(lineItem.sku).attr 'name', 'Order.Items.' + index + '.Variant.SKU'
#   el.find('a.title').text lineItem.name
#   el.find('div.variant-info').text variantInfo.join(' / ')
#   el.find('.price span').text formatCurrency(lineItem.price)
#   $quantity.val(lineItem.quantity).attr 'name', 'Order.Items.' + index + '.Quantity'

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
