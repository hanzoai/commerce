page = require 'page'

class Application
  constructor: (state = {}) ->
    @state = state

  # global setup
  setup: ->
    $.cookie.json = true

  # setup routing
  setupRouting: ->
    for k,v of @routes
      if Array.isArray v
        page.apply k, v...
      else
        page k, v

  start: ->
    @setupRoutes()
    page.start()

  get: (k) ->
    @state[k]

  set: (k, v) ->
    @state[k] = v

  delete: (k) ->
    delete @state[k]

module.exports = (state) ->
  app = new Application state
  app.setup()
  app
