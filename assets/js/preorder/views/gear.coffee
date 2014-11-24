{CategoryView, ItemView} = require './category'
View = require 'mvstar/lib/view'

products = require '../../utils/products'

$menSize = $('''<option value="XS">XS</option>
                <option value="S">S</option>
                <option value="M">M</option>
                <option value="L">L</option>
                <option value="XL">XL</option>
                <option value="XXL">XXL</option>
                <option value="XXXL">XXXL</option>''')

$womenSize = $('''<option value="XS">XS</option>
                  <option value="S">S</option>
                  <option value="M">M</option>
                  <option value="L">L</option>
                  <option value="XL">XL</option>''')

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

      # switch options
      $select = @el.find 'select.size'
      if style == "Men's T-Shirt"
        $.select.html().append $menSize
      else
        $.select.html().append $womenSize

      # force size to get set again
      @set 'size', (@get 'size')

      @set 'sku', variant.SKU

    'change select.size': (e, el) ->
      color = 'Black'
      style = @get 'style'
      size  = $(el).val()
      slug  = @get 'slug'

      @set 'size', size

      variant = products.getVariant slug,
        Color: color
        Size:  size
        Style: style

      @set 'sku', variant.SKU

class GearView extends CategoryView
  ItemView: GearItemView
  itemDefaults:
    sku:      'SKULLY-TSHIRT-MEN-M'
    slug:     't-shirt'
    style:    "Men's T-Shirt"
    quantity: 1
    size:     'M'
  name: 'gear'

  constructor: ->
    super
    @set 'title', 'Skully Nation Gear color & size'

module.exports = GearView
