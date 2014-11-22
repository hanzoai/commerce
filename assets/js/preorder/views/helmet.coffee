View = require 'mvstar/lib/view'

# Base class
class CategoryView extends Viewi
  index: 0
  ItemView: View
  itemDefaults: {}
  itemViews: []

  template:"#-template"
  bindings:
    count:      'span.counter' #array of counts
    total:      'span.total' #total number of things in category

  constructor: ->
    super
    @set 'count', 0
    @itemViews = []

  formatters:
    count: (v) ->
      count = @get 'count'
      if count != @get 'total'
        @el.find('span.counter').addClass 'bad'
      else
        @el.find('span.counter').removeClass 'bad'
      return count
    total: (v) ->
      return '/' + v + ')'

  events:
    updateCount: 'updateCount'
    newItem: 'newItem'
    deleteItem: 'deleteItem'

  updateCount: (index, newCount) ->
    counts = @get 'counts'
    counts[index] = newCount
    @set 'counts', counts

    #cancel bubbling
    return false

  newItem: ->
    @index++
    @itemViews[@index] = new @ItemView $.extend({index: @index}, @itemDefaults)
    #cancel bubbling
    return false

  deleteItem: ->

class HelmetView extends CategoryView
  template: '#helmet-template'
  ItemView: HelmetItemView
  itemDefaults:
    sku: ''
    slug: ''
    quantity: 0
    color: ''
    size: ''

class HelmetItemView extends View
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
          '#{v}.Variant.SKU'
        when 'input.slug @name'
          '#{v}.Product.Slug'
        when 'select.quantity @name'
          '#{v}.Quantity'
        when 'button.sub'
          return if v != 0 then '-' else ''

  events:
    # Dismiss on click, escape, and scroll
    'change select.quantity': 'updateQuantity'

    # Handle lineItem removals
    'click button.sub': ->
      @destroy() if @get 'index' != 0

    'click button.add': ->
      @trigger 'newItem'

  updateQuantity: (e) ->
    @trigger 'updateCount', parseInt $(e.currentTarget).val(), 10

  destroy: ->
    @unbind()
    @trigger 'removeItem', @get 'index'
    @$el.animate {opacity: "toggle"}, 500, 'swing', => @$el.remove()

module.exports = HelmetView
