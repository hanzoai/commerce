View = require 'mvstar/lib/view'

class HelmetView extends View
  template: '#helmet-template'

  bindings:
    sku:        'input.sku       @value'
    slug:       'input.slug      @value'
    quantity:   'select.quantity @value'
    color:      'select.color    @value'
    size:       'select.size     @value'
    index:     ['input.sku       @name'
                'input.slug      @name'
                'select.quantity @value']

  formatters:
    index: (v, selector) ->
      switch selector
        when 'input.sku @name'
          "Order.Items.#{v}.Variant.SKU"
        when 'input.slug @name'
          "Order.Items.#{v}.Product.Slug"
        when 'select.quantity @value'
          "Order.Items.#{v}.Quantity"

  events:
    # Dismiss on click, escape, and scroll
    'change .quantity input': 'updateQuantity'

    # Prevent user pressing enter
    'keypress input,select': (e) ->
      if e.keyCode isnt 13
        true
      else
        @updateQuantity(e)
        false

    # Handle lineItem removals
    'click .remove-item': ->
      @destroy()

  updateQuantity: (e) ->
    el = $(e.currentTarget)
    e.preventDefault()
    e.stopPropagation()

    # Get quantity
    quantity = parseInt(el.val(), 10)

    # Prevent less than one quantity
    if quantity < 1 || isNaN quantity
      quantity = 1

    # Update quantity
    @set 'quantity', quantity

  destroy: ->
    @unbind()
    @$el.animate {opacity: "toggle"}, 500, 'swing', => @$el.remove()

module.exports = HelmetView
