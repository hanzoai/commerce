{CategoryView, ItemView} = require './category'

class GearItemView extends ItemView
  template: '#gear-item-template'

  bindings:
    sku:        'input.sku       @value'
    slug:       'input.slug      @value'
    quantity:   'select.quantity @value'
    size:       'select.size     @value'
    style:      'select.style    @value'
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

  events: $.extend {}, ItemView::events,
    'change select.style': (e, el) ->
      color = 'Black'  # Always black my friend
      size  = @get 'size'
      slug  = @get 'slug'
      style = $(el).val()

      @set 'style', style

      variant = products.getVariant slug,
        Color: color
        Size:  size
        Style: style

      @set 'sku', variant.SKU

    'change select.size': (e, el) ->
      color = @get 'color'
      size  = $(el).val()
      slug  = @get 'slug'

      @set 'size', size

      variant = products.getVariant slug,
        Color: color
        Size:  size

      @set 'sku', variant.SKU

class GearView extends CategoryView
  ItemView: GearItemView
  itemDefaults:
    sku:      'SKULLY-TSHIRT-MEN-M'
    slug:     't-shirt'
    style:    "Men's Shirt"
    quantity: 1
    size:     'M'
  name: 'gear'

  constructor: ->
    super
    @set 'title', 'Skully Nation Gear color & size'

module.exports = GearView
