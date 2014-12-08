View = require 'mvstar/lib/view'
util = require '../util'

validNum = (v) ->
  # yes javascript YES YESSSSS YESSsssssssss
  typeof v == 'number' and not isNaN v

neverBelowOne = (v) ->
  if v < 1 then 1 else v

class LineSubItemView extends View
  template: '#line-subitem-template'

  bindings:
    img:        'img.thumbnail   @src'
    sku:        'input.sku       @value'
    slug:       'input.slug      @value'
    name:       'div.title'
    desc:       'div.desc'
    price:      '.price .money'
    quantity:  ['.quantity input @value'
                '.quantity .text']
    index:     ['input.sku       @name'
                'input.slug      @name'
                '.quantity input @name']

  computed:
    desc: (color, size) -> [color, size]

  watching:
    desc: ['color', 'size']

  formatters:
    desc: (v) ->
      if v.length > 1
        val = v.join ' / '
        if val == ' / '
          return ''
        return val
      else
        v.join ''

    quantity: (v)->
      v *= @get 'multiplier'

    index: (v, selector) ->
      switch selector
        when 'input.sku @name'
          "Order.Items.#{v}.Variant.SKU"
        when 'input.slug @name'
          "Order.Items.#{v}.Product.Slug"
        when '.quantity select @name'
          "Order.Items.#{v}.Quantity"

    price: (v) ->
      if v == 0
        @el.find('.price span').hide()
        @el.find('.price .free').show()
      else
        @el.find('.price span').show()
        @el.find('.price .free').hide()
      util.formatCurrency v

  destroy: ->
    @unbind()
    @$el.animate {opacity: "toggle"}, 500, 'swing', => @$el.remove()

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
      (app.get 'cart').removeProduct @state.listingSKU
      @destroy()

  render: ->
    super
    @children = []

    children = @state.children
    if children?
      for product in children
        view = new LineSubItemView state: product
        @el.parent().append view.el
        view.render()
        view.bind()
        view.set 'quantity', @get('quantity')
        @children.push view

  updateQuantity: (e, el) ->
    # Get quantity
    quantity = parseInt $(el).val(), 10
    console.log e, el, quantity

    # ensure sane input
    unless validNum quantity
      quantity = 1
    quantity = neverBelowOne quantity

    # Since this is LITERALLY the object in the cart, it fucks up tremendously
    # unless we clone our state object.
    @state = $.extend {}, @state

    # Update quantity
    @set 'quantity', quantity
    for child in @children
      child.set 'quantity', quantity

    # Update line item
    (app.get 'cart').setProduct @state.listingSKU, @state

  destroy: ->
    @unbind()
    @$el.animate {opacity: "toggle"}, 500, 'swing', => @$el.remove()
    for child in @children
      child.destroy()

module.exports = LineItemView
