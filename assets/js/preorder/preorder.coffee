App       = require 'mvstar/lib/app'
ErrorView = require './views/error'
routes    = require './routes'

class PreorderApp extends App
  prefix: '/:preorder?'

  routes:
    '/order/:token': [
      routes.order.initializeShipping
      routes.order.displayPerks
      routes.order.displayHelmets
      routes.order.displayApparel
      routes.order.displayHats
    ]

window.app = app = new PreorderApp()

app.route()

$(document).ready ->
  # Ensure that perk count matches configured perks
  $('.submit input[type=submit]').on 'click', ->
    # Clear any existing errors
    $('#errors').html('')

    perkCount  = ($('.counter').map (i,v) -> $(v).text()).toArray().join ''
    totalPerks = ($('.total').map (i,v) -> $(v).text()).toArray().join ''

    if perkCount != totalPerks
      view = new ErrorView()
      view.set 'message', "Your configured perks don't match your preorder."
      view.set 'link',    '#ar1'
      view.render()
      $('#errors').append view.el
      return false

  # Form validation
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
    for error in errors
      $('#' + error.id).addClass 'fix'

      # Append error message
      view = new ErrorView()
      view.set 'message', error.message
      view.set 'link',    '#' + error.id
      view.render()

      $('#errors').append view.el

  validator.registerCallback 'numeric_dash', (value) ->
    (new RegExp /^[\d\-\s]+$/).test value
