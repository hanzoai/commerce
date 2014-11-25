App       = require 'mvstar/lib/app'
ErrorView = require './views/error'
routes    = require './routes'

displayErrors = (errors = {}) ->
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

    $('.loading-spinner').removeClass('loading-spinner')
  setTimeout(->
    $('.submit').removeAttr('disabled')
  , 10)

setupValidation = ->
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
  ], displayErrors

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

showPreorderForm = ->
  $('.password-form').hide()
  $('.shipping, .perk, .item, .submitter').show()
  setupValidation()
  displayErrors()

  $('form#skully').on 'submit', ->
    $('input.submit').attr('disabled', 'disabled')

# Disable enter
$('form').on 'keypress', (e) -> e.keyCode isnt 13

$(document).ready ->
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

  # prevent password form from submitting
  $('.password-form .submit').on 'submit', (e) -> false

  $('input.submit').on 'click', ->
    $('.save-spinner').addClass('loading-spinner')

  # Already visited, saved password
  if PreorderData.hasPassword
    $('.password-form').remove()
    return showPreorderForm()

  # New account
  $('.password-form').show()
  $('.password-form .submit').on 'click', (e) ->
    errors = []

    if $('#password').val().length < 6
      errors.push
        message: 'Your password must be at least 6 characters long.'
        id: 'password'

    if $('#password_confirm').val() != $('#password').val()
      errors.push
        message: 'The passwords you typed do not match.'
        id: 'password_confirm'

    # Show form if user managed to type a password (it's tough).
    if errors.length
      displayErrors errors
    else
      $('.next-spinner').addClass('loading-spinner')
      setTimeout(->
        showPreorderForm()
      , 500)

    false
