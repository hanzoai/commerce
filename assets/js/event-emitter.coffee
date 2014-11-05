class EventEmitter
  constructor: ->
    @_jQuery = $(@)

  emit: (event, data...) ->
    @_jQuery.trigger event, data

  once: (event, callback) ->
    @_jQuery.one event, (event, data...) =>
      callback.apply @, data

  on: (event, callback) ->
    @_jQuery.bind event, (event, data...) =>
      callback.apply @, data

  off: (event, callback) ->
    @_jQuery.unbind event, callback

module.exports = EventEmitter
