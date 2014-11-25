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
        when 'select.size @name'
          "Order.Items.#{v}.Size"
        when 'select.quantity @name'
          "Order.Items.#{v}.Quantity"

  events: $.extend {}, ItemView::events,
    'change select.color': (e, el) ->
      color = $(el).val()
      size  = @get 'size'
      slug  = @get 'slug'

      @set 'color', color

      variant = products.getVariant slug,
        Color: color
        Size:  size

      @set 'sku', variant.SKU
      @emit 'updateColor', color

    'change select.size': (e, el) ->
      color = @get 'color'
      size  = $(el).val()
      slug  = @get 'slug'

      @set 'size', size

      variant = products.getVariant slug,
        Color: color
        Size:  size

      @set 'sku', variant.SKU

class HelmetView extends CategoryView
  template: '#helmet-template'
  ItemView: HelmetItemView
  itemDefaults:
    sku:      'AR-1-BLACK-M'
    slug:     'ar-1'
    quantity: 1
    color:    'Matte Black'
    size:     'M'
  name: 'helmet'


  bindings: $.extend {}, HelmetView::bindings,
    color: 'div.thumbnail @src'

  formatters: $.extend {}, HelmetView::formatters,
    color: (v, selector)->
      if v == "Matte Black"
        @$el.find('div.thumbnail .black').animate({opacity: 1})
        @$el.find('div.thumbnail .white').animate({opacity: 0})
      else
        @$el.find('div.thumbnail .white').animate({opacity: 1})
        @$el.find('div.thumbnail .black').animate({opacity: 0})
      ''

  constructor: ->
    super
    @set 'title', 'Skully AR-1 color & size'
    @set 'color', 'Matte Black'

  newItem: ->
    isFirstItem = !@firstItemView
    itemView = super
    if isFirstItem
      @firstItemView.on('updateColor', (color)=> @set 'color', color)
    itemView

window.HelmetItemView = HelmetItemView

module.exports = HelmetView
