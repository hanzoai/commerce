View        = require 'mvstar/lib/view'
ViewEmitter = require 'mvstar/lib/view-emitter'

_index = 0

class CategoryView extends ViewEmitter
  ItemView:     View
  index:        0
  itemDefaults: {}
  itemViews:    []
  name:         'category'

  template: '#category-template'

  bindings:
    title:  'span.title'
    counts: ['span.counter'
            '.counter-validation @value']  # array of counts
    total:  'span.total'  # total number of things in category, SHOULD NOT CHANGE

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

  render: ->
    super
    name = @name + '-counter'
    @$el.find('.counter-validation').attr('name', name).attr('id', name)
    @$el.find('a.local-link').attr('name', name)

  updateCount: (data) ->
    counts = @get 'counts'
    counts[data.index] = data.count
    @set 'counts', counts

  newItem: ->
    _index++
    if @el.find('.form').children().length == @get 'total'
      return

    # Create new view instance
    itemView = new @ItemView
      total: @get 'total'
      state: $.extend({index: _index}, @itemDefaults)

    @firstItemView = itemView unless @firstItemView?

    # Listen to events on ItemView
    itemView.on 'newItem',     => @newItem.apply @, arguments
    itemView.on 'removeItem',  => @removeItem.apply @, arguments
    itemView.on 'updateCount', => @updateCount.apply @, arguments
    itemView.on 'updateCount', => @updateCount.apply @, arguments

    # Set initial count
    @updateCount
      index: itemView.get 'index'
      count: 1

    # Render and bind events
    itemView.render()
    if _index == 1
      itemView.$el.find('button.sub').remove()

    itemView.bind()
    @itemViews[@index] = itemView
    @el.find('.form:first').append itemView.$el

    if @el.find('.form').children().length == @get 'total'
      itemView.$el.find('button.add').remove()

    itemView

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

    'change select': ->
      @el.find

    # Handle lineItem removals
    'click button.sub': ->
      @destroy()

    'click button.add': ->
      @emit 'newItem'

  render: ->
    super
    quantity = @el.find '.quantity'
    for i in [1..@total]
      quantity.append $('<option/>').attr('value', i).text(i)

  updateQuantity: (e, el) ->
    @emit 'updateCount',
      index: (@get 'index')
      count: parseInt $(el).val(), 10

  destroy: ->
    @unbind()
    @emit 'removeItem', (@get 'index')
    @el.animate {height: '0px', opacity: 'toggle'}, 100, 'swing', => @el.remove()

module.exports =
  CategoryView: CategoryView
  ItemView:     ItemView
