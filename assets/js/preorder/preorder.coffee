App = require 'mvstar/lib/app'

class PreorderApp extends App
  start: ->
    super
    $.cookie.json = true

window.app = app = new PreorderApp()

# Store variant options for later
app.set 'variants', (require './variants')


app.routes =
  '/order/*': []

app.start()
