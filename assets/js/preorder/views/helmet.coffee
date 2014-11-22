Views = require './category'

class HelmetItemView extends Views.ItemView
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
                'button.sub']

  formatters:
    index: (v, selector) ->
      switch selector
        when 'input.sku @name'
          "#{v}.Variant.SKU"
        when 'input.slug @name'
          "#{v}.Product.Slug"
        when 'select.color @name'
          "#{v}.Color"
        when 'select.size @name'
          "#{v}.Size"
        when 'select.quantity @name'
          "#{v}.Quantity"
        when 'button.sub'
          return if v != 0 then '-' else ''

class HelmetView extends Views.CategoryView
  template: '#helmet-template'
  ItemView: HelmetItemView
  itemDefaults:
    sku: ''
    slug: ''
    quantity: 1
    color: ''
    size: ''

module.exports = HelmetView
