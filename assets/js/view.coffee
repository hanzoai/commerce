util = require './util'

class View
  constructor: (opts = {}) ->
    @el     ?= opts.el
    @$el    ?= $(@el)
    @id      = util.uniqueId @constructor.name
    @state   = opts.state ? {}
    @_events = {}

    @bindingsReverse = {}
    for k,v of @bindings
      @bindingsReverse[v] = k

    @_cacheDatabindEls()

    unless not opts.autoRender
      @render()

  get: (k) ->
    @state[k]

  set: (k, v) ->
    @state[k] = v
    @_databindEls[@bindingsReverse[k]].text v

  _cacheDatabindEls: ->
    return if @_databindEls?

    @_databindEls = {}

    for k,v of @bindings
      @_databindEls[k] = @$el.find k

  # render data bindings
  render: (state) ->
    # update state
    for k,v of state
      @state[k] = v

    # update text on all bindings
    for k,v of @bindings
      @_databindEls[k].text @state[v]

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
