App      = require 'mvstar/lib/app'
routes   = require './routes'

class PreorderApp extends App
  start: ->
    super

window.app = app = new PreorderApp()

# Store variant options for later
app.set 'variants', (require './variants')

app.routes =
  '/preorder/order/:token': [
    routes.order.initializeShipping
    routes.order.displayPerks
    routes.order.displayHelmets
    routes.order.displayApparel
    routes.order.displayHats
  ]
  '*': [
    (-> console.log 'global')
  ]

app.start()

$(document).ready ->

  $(".submit input[type=submit]").on "click", ->
    ret = true

    # there used to be mroe stuff here
    ret = validateCount() and ret
    ret

  validator = new FormValidator "skully", [
    {
      name: "email"
      rules: "required|valid_email"
    }
    {
      name: "password"
      rules: "required|min_length[6]"
    }
    {
      name: "password_confirm"
      display: "password confirmation"
      rules: "required|matches[password]"
    }
    {
      name: "first_name"
      display: "first name"
      rules: "required"
    }
    {
      name: "last_name"
      display: "last name"
      rules: "required"
    }
    {
      name: "phone"
      rules: "callback_numeric_dash"
    }
    {
      name: "address1"
      display: "address"
      rules: "required"
    }
    {
      name: "city"
      rules: "required"
    }
    {
      name: "postal_code"
      display: "postal code"
      rules: "required|numeric_dash"
    }
  ], (errors, event) ->
    i = 0

    while i < errors.length
      $("#" + errors[i].id).addClass "fix"
      i++
    return

  validator.registerCallback "numeric_dash", (value) ->
    (new RegExp(/^[\d\-\s]+$/)).test value
