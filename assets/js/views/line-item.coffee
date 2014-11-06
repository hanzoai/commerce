View = require '../view'
util = require '../util'

class LineItemView extends View
  template: '#line-item-template'

  bindings:
    img:        'img.thumbnail   @src'
    slug:       'input.slug      @value'
    name:       'a.title'
    desc:       'div.desc'
    price:      '.price span'
    quantity:   '.quantity input @value'

    index:     ['input.sku       @name'
                'input.slug      @name'
                '.quantity input @name']

    skuIndex:   'input.sku @name'
    slugIndex:  'input.slug @name'
    quantIndex: '.quantity input @name'

  computed:
    desc: (color, size) -> [color, size]

  watching:
    desc: ['color', 'size']

  formatters:
    slug: (v) ->
      'Order.Items.' + v + '.Product.Slug'

    index: (v, selector) ->
      switch selector
        when 'input.sku @name'
          "Order.Items.#{v}.Variant.SKU"
        when 'input.slug @name'
          "Order.Items.#{v}.Product.Slug"

    desc: (v) ->
      v.join ' / '

    price: (v) ->
      util.formatCurrency v

module.exports = LineItemView
