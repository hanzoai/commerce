{CategoryView, ItemView} = require './category'
products = require '../../utils/products'

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

  events:
    'change select.color': (e, el) ->
      color   = $(el).val()
      size    = @get 'size'
      slug    = @get 'slug'
      variant = products.getVariant slug,
        Color: color
        Size:  size

      @set 'sku', variant.SKU
      console.log variant

    'change select.size': (e, el) ->
      color   = @get 'color'
      size    = $(el).val()
      slug    = @get 'slug'
      console.log color, size, slug

      variant = products.getVariant slug,
        Color: color
        Size:  size

      @set 'sku', variant.SKU
      console.log variant

class HelmetView extends CategoryView
  ItemView: HelmetItemView
  itemDefaults:
    sku:      'AR-1-BLACK-S'
    slug:     'ar-1'
    quantity: 1
    color:    'Matte Black'
    size:     'S'

  constructor: ->
    super
    @set 'title', 'Skully AR-1 color & size'

module.exports = HelmetView
