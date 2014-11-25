{CategoryView, ItemView} = require './category'

products = require '../../utils/products'

class HatsItemView extends ItemView
  template: '#hat-item-template'

  bindings:
    sku:        'input.sku       @value'
    slug:       'input.slug      @value'
    quantity:   'select.quantity @value'
    size:       'select.size     @value'
    index:     ['input.sku       @name'
                'input.slug      @name'
                'select.size     @name'
                'select.quantity @name']

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

  events: $.extend {}, ItemView::events,
    'change select.size': (e, el) ->
      size = $(el).val()
      variant = products.getVariant (@get 'slug'), Size: size
      @set 'sku', variant.SKU


class HatsView extends CategoryView
  template: '#hat-template'
  ItemView: HatsItemView
  itemDefaults:
    slug:     'hat'
    sku:      'SKULLY-HAT-M'
    quantity: 1
    size:     'M'

  name: 'hat'

module.exports = HatsView
