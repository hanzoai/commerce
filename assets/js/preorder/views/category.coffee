View = require 'mvstar/lib/view'

class EmitterView extends View
  emitter: null
  constructor: (opts)->
    super
    @emitter = opts.emitter

class CategoryView extends EmitterView
  index: 0
  ItemView: View
  itemDefaults: {}
  itemViews: []

  template:"#-template"
  bindings:
    counts:     'span.counter' #array of counts
    total:      'span.total' #total number of things in category, SHOULD NOT CHANGE

  constructor: ->
    super
    @set 'counts', []
    @itemViews = []

    @emitter.on 'updateCount', => @updateCount.apply(@, arguments)
    @emitter.on 'newItem', => @newItem.apply(@, arguments)
    @emitter.on 'removeItem', => @removeItem.apply(@, arguments)

  formatters:
    counts: (v) ->
      count = (@get 'counts').reduce ((sum, n)-> return sum + n), 0
      if count != @get 'total'
        @$el.find('span.counter').addClass 'bad'
      else
        @$el.find('span.counter').removeClass 'bad'
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
      emitter: @emitter,
      total: @get 'total'
      state: $.extend({index: @index}, @itemDefaults)

    itemView.render()
    itemView.bind()
    @itemViews[@index] = itemView
    @$el.find('.form:first').append itemView.$el
    #cancel bubbling
    return false

  removeItem: (index)->
    counts = @get 'counts'
    counts[index] = 0
    @set 'counts', counts
    @updateCount {index: index, count: 0}

class ItemView extends EmitterView
  total: 1
  constructor: (opts)->
    super
    @total = opts.total
    @emitter.emit 'updateCount', {index: @get('index'), count: 1}

  events:
    # Dismiss on click, escape, and scroll
    'change select.quantity': 'updateQuantity'

    # Handle lineItem removals
    'click button.sub': ->
      @destroy() if @get('index') > 0

    'click button.add': ->
      @emitter.emit 'newItem'

  render: ()->
    super
    quantity = @$el.find('.quantity')
    for i in [1..@total]
      quantity.append $('<option/>').attr('value', i).text(i)

  updateQuantity: (e) ->
    @emitter.emit 'updateCount', {index: @get('index'), count: parseInt $(e.currentTarget).val(), 10}

  destroy: ->
    @unbind()
    @emitter.emit 'removeItem', @get 'index'
    @$el.animate {height: "0px", opacity: "toggle"}, 100, 'swing', => @$el.remove()

module.exports =
  CategoryView: CategoryView
  ItemView: ItemView
