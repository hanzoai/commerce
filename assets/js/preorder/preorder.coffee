App       = require 'mvstar/lib/app'
routes    = require './routes'
window.ErrorView = require './views/error'

class PreorderApp extends App
  start: ->
    @route()

window.app = app = new PreorderApp()

# Store variant options for later
app.set 'variants', (require './variants')

app.routes =
  '/:prefix?/order/:token': [
    routes.order.initializeShipping
    routes.order.displayPerks
    routes.order.displayHelmets
    routes.order.displayApparel
    routes.order.displayHats
  ]

app.start()

$(document).ready ->
  validator = new FormValidator 'skully', [
      name: 'email'
      rules: 'required|valid_email'
    ,
      name: 'password'
      rules: 'required|min_length[6]'
    ,
      name: 'password_confirm'
      display: 'password confirmation'
      rules: 'required|matches[password]'
    ,
      name: 'first_name'
      display: 'first name'
      rules: 'required'
    ,
      name: 'last_name'
      display: 'last name'
      rules: 'required'
    ,
      name: 'phone'
      rules: 'callback_numeric_dash'
    ,
      name: 'address1'
      display: 'address'
      rules: 'required'
    ,
      name: 'city'
      rules: 'required'
    ,
      name: 'postal_code'
      display: 'postal code'
      rules: 'required|numeric_dash'
  ], (errors, event) ->
    # Clear any existing errors
    $('#errors').html('')

    for error in errors
      $('#' + error.id).addClass 'fix'

      # Append error message
      view = new ErrorView()
      view.set 'message', error.message
      view.set 'link',    '#' + error.id
      view.render()

      $('#errors').append view.el

  validator.registerCallback 'numeric_dash', (value) ->
    (new RegExp(/^[\d\-\s]+$/)).test value
