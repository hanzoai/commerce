App       = require 'mvstar/lib/app'
routes    = require './routes'
window.ErrorView = require './views/error'

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

# Store variant options for later
app.set 'variants', (require './variants')

app.route()

$(document).ready ->
  if PreorderData.hasPassword
    $('.shipping, .perk, .item, .submitter').show()
  else
    $('.password-form').show()
    $('.password-form .submit').on 'click', ->
      $('.shipping, .perk, .item, .submitter').show()
      $('.password-form').hide()

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
      rules: 'required|alpha_dash'
    ,
      name: 'helmet-counter'
      rules: 'callback_check_helmet_counter'
    ,
      name: 'gear-counter'
      rules: 'callback_check_gear_counter'
    ,
      name: 'hat-counter'
      rules: 'callback_check_hat_counter'
  ], (errors, event) ->
    $('.errors').html('') # Clear any existing errors
    for error in errors
      $('#' + error.id).addClass 'fix'
      $('#' + error.id).parent().find('.quantity').addClass 'fix'

      # Append error message
      view = new ErrorView()
      view.set 'message', error.message
      view.set 'link',    '#' + error.id
      view.render()

      $('.errors').append view.el

  validator.registerCallback 'numeric_dash', (value) ->
    (new RegExp /^[\d\-\s]+$/).test value

  validator.registerCallback('check_helmet_counter', (value) ->
    return window.helmetTotal == parseInt value, 10 #set in routes/order
  ).setMessage('check_helmet_counter', "Your helmet choices don't match your preorder.")

  validator.registerCallback('check_gear_counter', (value) ->
    return window.gearTotal == parseInt value, 10 #set in routes/order
  ).setMessage('check_gear_counter', "Your gear choices don't match your preorder.")

  validator.registerCallback('check_hat_counter', (value) ->
    return window.gearTotal == parseInt value, 10 #set in routes/order
  ).setMessage('check_hat_counter', "Your hat choices don't match your preorder.")

