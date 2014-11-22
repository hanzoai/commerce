View = require 'mvstar/lib/view'

class CategoryView extends View
  index: 0
  ItemView: View
  itemDefaults: {}
  itemViews: []

  template:"#-template"
  bindings:
    count:      'span.counter' #array of counts
    total:      'span.total' #total number of things in category, SHOULD NOT CHANGE

  constructor: ->
    super
    @set 'count', 0
    @itemViews = []

  formatters:
    count: (v) ->
      count = @get 'count'
      if count != @get 'total'
        @$el.find('span.counter').addClass 'bad'
      else
        @$el.find('span.counter').removeClass 'bad'
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
    itemView = new @ItemView {total: @get 'total', state: $.extend({index: @index}, @itemDefaults)}
    itemView.render()
    @itemViews[@index] = itemView
    @$el.find('.form:first').append itemView.$el
    #cancel bubbling
    return false

  deleteItem: ->

class ItemView extends View
  total: 1
  constructor: (opts)->
    super
    @total = opts.total

  events:
    # Dismiss on click, escape, and scroll
    'change select.quantity': 'updateQuantity'

    # Handle lineItem removals
    'click button.sub': ->
      @destroy() if @get 'index' != 0

    'click button.add': ->
      @trigger 'newItem'

  render: ()->
    super
    quantity = @$el.find('.quantity')
    for i in [1..@total]
      quantity.append $('<option/>').attr('value', i).text(i)

  updateQuantity: (e) ->
    @trigger 'updateCount', parseInt $(e.currentTarget).val(), 10

  destroy: ->
    @unbind()
    @trigger 'removeItem', @get 'index'
    @$el.animate {opacity: "toggle"}, 500, 'swing', => @$el.remove()

module.exports =
  CategoryView: CategoryView
  ItemView: ItemView
