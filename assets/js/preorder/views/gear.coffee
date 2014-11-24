{CategoryView, ItemView} = require './category'

class GearItemView extends ItemView
  template: '#gear-item-template'

  bindings:
    sku:        'input.sku       @value'
    slug:       'input.slug      @value'
    quantity:   'select.quantity @value'
    size:       'select.size     @value'
    index:     ['input.sku       @name'
                'input.slug      @name'
                'select.style    @name'
                'select.size     @name'
                'select.quantity @name'
                'button.sub      @text']

  formatters:
    index: (v, selector) ->
      switch selector
        when 'input.sku @name'
          "Order.Items.#{v}.Variant.SKU"
        when 'input.slug @name'
          "Order.Items.#{v}.Product.Slug"
        when 'select.color @name'
          "Order.Items.#{v}.Color"
        when 'select.style @name'
          "Order.Items.#{v}.Style"
        when 'select.size @name'
          "Order.Items.#{v}.Size"
        when 'select.quantity @name'
          "Order.Items.#{v}.Quantity"
        when 'button.sub @text'
          if v > 1
            '-'
          else
            ''

class GearView extends CategoryView
  ItemView: GearItemView
  itemDefaults:
    sku: ''
    slug: ''
    quantity: 1
    size: ''
  name: 'gear'

  constructor: ->
    super
    @set 'title', 'Skully Nation Gear color & size'

module.exports = GearView
