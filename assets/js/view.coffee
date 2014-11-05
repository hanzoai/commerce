util = require './util'

class View
  el:         null
  formatters: {}
  events:     {}
  bindings:   {}

  constructor: (opts = {}) ->
    @el ?= opts.el

    # You can get an element for the view multiple ways:
    # 1. Pass it in as $el
    # 2. Use a template to create a new element.
    # 3. Find it in DOM using @el selector.
    if opts.$el
      @$el = opts.$el
    else
      if @template
        @$el = $($(@template).html())
      else
        @$el = $(@el)

    @id         = util.uniqueId @constructor.name
    @state      = opts.state ? {}
    @_events    = {}
    @_databinds = {}

    # find all elements in DOM.
    @_cacheDatabinds()

    unless not opts.autoRender
      @render()

  _cacheDatabinds: ->
    for k,v of @bindings
      @_databinds[k] = $(@$el.find v)

  _updateBinding: (k, v) ->
    if (formatter = @formatters[k])?
      v = formatter v
    @_databinds[k].text v

  get: (k) ->
    @state[k]

  set: (k, v) ->
    @state[k] = v
    @_updateBinding k, v

  render: (state) ->
    # update state
    for k,v of state
      @set k, v
    @

  _splitEvent: (event) ->
    [event, selector] = event.split /\s+/

    unless selector
      $el = @$el
      return [$el, event]

    # allow global event binding
    switch selector
      when 'document'
        $el = $(document)
      when 'window'
        $el = $(window)
      else
        $el = @$el.find selector

    [$el, event]

  on: (event, callback) ->
    [$el, event] = @_splitEvent event
    $el.on "#{event}.#{@id}", (event, data...) =>
      callback.apply @, data
    @

  once: (event, callback) ->
    [$el, event] = @_splitEvent event
    $el.one "#{event}.#{@id}", (event, data...) =>
      callback.apply @, data
    @

  off: (event) ->
    [$el, event] = @_splitEvent event
    $el.off "#{event}.#{@id}"
    @

  emit: (event, data...) ->
    [$el, event] = @_splitEvent event
    $el.trigger event, data
    @

  bind: ->
    @on k,v for k,v of @events
    @

  unbind: ->
    @off k,v for k,v of @events
    @

module.exports = View
