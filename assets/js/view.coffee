util = require './util'

class View
  constructor: (opts = {}) ->
    @el     ?= opts.el
    @$el    ?= $(@el)
    @id      = util.uniqueId @constructor.name
    @state   = opts.state ? {}
    @_events = {}

    unless not opts.autoRender
      @render()

  _cacheDatabinds: ->
    return if @_databinds?

    @_databinds = {}

    for k,v of @bindings
      @_databinds[k] = $(@$el.find v)

  updateBinding: (k, v) ->
    if (formatter = @formatters[k])?
      v = formatter v
    @databinds[k].text v

  get: (k) ->
    @state[k]

  set: (k, v) ->
    @state[k] = v
    @updateBinding k, v

  render: (state) ->
    # update state
    for k,v of state
      @set k, v

  _splitEvent: (event) ->
    [event, selector] = event.split /\s+/

    unless selector
      $el = @$el
      return [$el, event]

    # allow global event binding
    if /^document$|^window$/.test selector
      $el = $(selector)
    else
      $el = @$el.find selector

    [$el, event]

  # bind event namespaced to view id
  on: (event, callback) ->
    @_events[event] = callback
    [$el, eventName] = @_splitEvent event
    $el.on "#{event}.#{@id}", => callback.apply @, arguments

  # unbind event
  off: (event) ->
    if event
      callback = @_events[event]
      [$el, event] = @_splitEvent event
      $el.off "#{event}.#{@id}", callback
    else
      for k,v of @_events
        [$el, event] = @_splitEvent k
        $el.off "#{event}.#{@id}", v

  trigger: (event, params...) ->
    [$el, event] = @_splitEvent event
    $el.trigger event, params...

  bind: ->
    @on k,v for k,v of @events
    return

  unbind: ->
    @off k,v for k,v of @events
    return

module.exports = View
