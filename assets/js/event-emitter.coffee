class EventEmitter
  constructor: ->
    @_jQuery = $(@)

  emit: (event, data) ->
    @_jQuery.trigger event, data

  once: (event, handler) ->
    @_jQuery.one event, handler

  on: (event, handler) ->
    @_jQuery.bind event, handler

  off: (event, handler) ->
    @_jQuery.unbind event, handler

module.exports = EventEmitter
