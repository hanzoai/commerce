View = require 'mvstar/lib/view'
util = require '../util'

validNum = (v) ->
  # yes javascript YES YESSSSS YESSsssssssss
  typeof v == 'number' and not isNaN v

neverBelowOne = (v) ->
  if v < 1 then 1 else v

class LineItemView extends View
  template: '#line-item-template'

  bindings:
    img:        'img.thumbnail   @src'
    sku:        'input.sku       @value'
    slug:       'input.slug      @value'
    name:       'div.title'
    desc:       'div.desc'
    price:      '.price .money'
    quantity:   '.quantity select @value'
    index:     ['input.sku       @name'
                'input.slug      @name'
                '.quantity select @name']

  computed:
    desc: (color, size) -> [color, size]

  watching:
    desc: ['color', 'size']

  formatters:
    desc: (v) ->
      if v.length > 1
        v.join ' / '
      else
        v.join ''

    index: (v, selector) ->
      switch selector
        when 'input.sku @name'
          "Order.Items.#{v}.Variant.SKU"
        when 'input.slug @name'
          "Order.Items.#{v}.Product.Slug"
        when '.quantity select @name'
          "Order.Items.#{v}.Quantity"

    price: (v) ->
      util.formatCurrency v

  events:
    'change .quantity select': 'updateQuantity'

    # 'keypress .quantity input': (e, el) ->
    #   @set el

    # Prevent user pressing enter
    'keypress input,select': (e, el) ->
      if e.keyCode isnt 13
        true
      else
        @updateQuantity e, el
        false

    # Handle lineItem removals
    'click .remove-item': ->
      (app.get 'cart').removeProduct @state.sku
      @destroy()

  updateQuantity: (e, el) ->
    # Get quantity
    quantity = parseInt $(el).val(), 10
    console.log e, el, quantity

    # ensure sane input
    unless validNum quantity
      quantity = 1
    quantity = neverBelowOne quantity

    cart = app.get 'cart'
    if (cart.getProduct @state.sku).quantity + quantity > (app.get 'maxQuantityPerProduct')
      console.log 'TOO MANY THINGS IN CART'
      return

    # Since this is LITERALLY the object in the cart, it fucks up tremendously
    # unless we clone our state object.
    @state = $.extend {}, @state

    # Update quantity
    @set 'quantity', quantity


    # Update line item
    (app.get 'cart').setProduct @state.sku, @state

  destroy: ->
    @unbind()
    @$el.animate {opacity: "toggle"}, 500, 'swing', => @$el.remove()

module.exports = LineItemView
