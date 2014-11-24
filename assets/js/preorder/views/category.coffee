View        = require 'mvstar/lib/view'
ViewEmitter = require 'mvstar/lib/view-emitter'

class CategoryView extends ViewEmitter
  index: 0
  ItemView: View
  itemDefaults: {}
  itemViews: []

  template:"#category-template"

  bindings:
    title:  'span.title'
    counts: 'span.counter' # array of counts
    total:  'span.total'   # total number of things in category, SHOULD NOT CHANGE

  constructor: ->
    super
    @set 'counts', []
    @itemViews = []

  formatters:
    title: (v)->
      return v + ' '
    counts: (v) ->
      count = (@get 'counts').reduce ((sum, n)-> return sum + n), 0
      if count != @get 'total'
        @el.find('span.counter').addClass 'bad'
      else
        @el.find('span.counter').removeClass 'bad'
      return count
    total: (v) ->
      return '/' + v + ')'

  updateCount: (data) ->
    counts = @get 'counts'
    counts[data.index] = data.count
    @set 'counts', counts

    #cancel bubbling
    return false

  newItem: ->
    @index++

    itemView = new @ItemView
      total: @get 'total'
      state: $.extend({index: @index}, @itemDefaults)

    itemView.on 'newItem',     => @newItem.apply @, arguments
    itemView.on 'removeItem',  => @removeItem.apply @, arguments
    itemView.on 'updateCount', => @updateCount.apply @, arguments
    @updateCount
      index: itemView.get('index')
      count: 1

    itemView.render()
    itemView.bind()
    @itemViews[@index] = itemView
    @el.find('.form:first').append itemView.$el

    return false  # cancel bubbling

  removeItem: (index) ->
    counts = @get 'counts'
    counts[index] = 0
    @set 'counts', counts
    @updateCount
      index: index
      count: 0

class ItemView extends ViewEmitter
  total: 1

  constructor: (opts) ->
    super
    @total = opts.total

  events:
    # Dismiss on click, escape, and scroll
    'change select.quantity': 'updateQuantity'

    # Handle lineItem removals
    'click button.sub': ->
      @destroy() if @get('index') > 0

    'click button.add': ->
      @emit 'newItem'

  render: ->
    super
    quantity = @el.find('.quantity')
    for i in [1..@total]
      quantity.append $('<option/>').attr('value', i).text(i)

  updateQuantity: (e) ->
    @emit 'updateCount',
      index: (@get 'index')
      count: parseInt $(e.currentTarget).val(), 10

  destroy: ->
    @unbind()
    @emit 'removeItem', (@get 'index')
    @el.animate {height: '0px', opacity: 'toggle'}, 100, 'swing', => @el.remove()

module.exports =
  CategoryView: CategoryView
  ItemView:     ItemView
