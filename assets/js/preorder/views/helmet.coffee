{CategoryView, ItemView} = require './category'
products = require '../../utils/products'

events = ItemView::events

events['change select.color'] = (e, el) ->
  color   = $(el).val()
  @set 'color', color

  size    = @get 'size'
  slug    = @get 'slug'

  console.log slug, color, size
  variant = products.getVariant slug,
    Color: color
    Size:  size

  @set 'sku', variant.SKU

events['change select.size'] = (e, el) ->
  size    = $(el).val()
  @set 'size', size

  color   = @get 'color'
  slug    = @get 'slug'

  console.log slug, color, size
  variant = products.getVariant slug,
    Color: color
    Size:  size

  @set 'sku', variant.SKU

class HelmetItemView extends ItemView
  template: '#helmet-item-template'

  bindings:
    sku:        'input.sku       @value'
    slug:       'input.slug      @value'
    quantity:   'select.quantity @value'
    color:      'select.color    @value'
    size:       'select.size     @value'
    index:     ['input.sku       @name'
                'input.slug      @name'
                'select.color    @name'
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
        when 'select.size @name'
          "Order.Items.#{v}.Size"
        when 'select.quantity @name'
          "Order.Items.#{v}.Quantity"
        when 'button.sub @text'
          if v > 1
            '-'
          else
            ''

  events: events

class HelmetView extends CategoryView
  ItemView: HelmetItemView
  itemDefaults:
    sku:      'AR-1-BLACK-M'
    slug:     'ar-1'
    quantity: 1
    color:    'Matte Black'
    size:     'M'
  name: 'helmet'

  constructor: ->
    super
    @set 'title', 'Skully AR-1 color & size'

module.exports = HelmetView
