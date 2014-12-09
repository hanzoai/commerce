util = require '../store/util'
require 'card'
validator = require 'address-validator/src/validator'

# Validation helper
validation =
  isEmpty: (str) ->
    str.trim().length is 0

  isEmail: (email) ->
    pattern = new RegExp(/^[+a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,4}$/i)
    pattern.test email

validateForm = ->
  $errors = $('#error-message')
  $errors.text ''

  valid = true
  errors = []

  # Get all inputs that are visible and empty
  empty = $('div:visible.required > input').filter ->
    validation.isEmpty $(@).val()

  window.empty = empty

  email = $('input[name="User.Email"]')
  if email.length != 0
    unless validation.isEmail email.val()
      valid = false
      email.addClass 'error'
      email.addClass 'shake'
      setTimeout ->
        email.removeClass 'shake'
      , 500
      errors.push "Invalid email."

  if empty.length > 0
    valid = false
    empty.addClass 'error'
    empty.addClass 'shake'
    setTimeout ->
      empty.removeClass 'shake'
      return
    , 500
    missing = (v.trim() for v in empty.parent().text().split('\n') when v.trim())
    errors.push "Missing #{missing.join ', '}."

  unless valid
    # display errors
    for error in errors
      $errors.append $("<p>#{error}</p>")

    # scroll to first error
    if empty.length > 0
      location.href = '#' + empty[0].id
      pos = $(window).scrollTop() - 100 # save position
      location.hash = '' # clear hash
      $(window).scrollTop pos

  return valid

# Remove error class from field that has been edited or clicked on
clearError = -> $(@).removeClass 'error'
$('div.field input').on 'click', clearError
$('div.field input').on 'change', clearError

$('input[name="ShipToBilling"]').change ->
  shipping = $('.shipping-information fieldset')
  if @checked
    shipping.fadeOut 500
    setTimeout ->
      shipping.css 'display', 'none'
      return
    , 500
  else
    shipping.fadeIn 500
    shipping.css 'display', 'block'
  return


# Update tax display
$state    = $('input[name="Order.BillingAddress.State"]')
$city     = $('input[name="Order.BillingAddress.City"]')

$subtotal = $('span.subtotal')
$tax      = $('span.tax')
$shipping = $('span.shipping')
$total    = $('span.grand-total')
$country  = $('input[name="Order.BillingAddress.Country"]')

updateShippingAndTax = $.debounce 250, ->
  country  = $country.val().trim().replace ' ', ''
  city     = $city.val().trim()
  state    = $state.val().trim()

  subtotal = parseFloat $subtotal.text().replace ',', ''
  shipping = 0
  tax      = 0
  total    = 0

  # Update shipping
  unless (/^usa$|^us$|unitedstates$|unitedstatesofamerica/i).test country
    shipping = 100.00
  else
    shipping = 0

  # Update tax
  if ((/^usa$|^us$|unitedstates$|unitedstatesofamerica/i).test country) and
     (/^ca$|^cali/i).test state
    # Add CA tax
    tax += subtotal * 0.075
    # Add SF county tax
    tax += subtotal * 0.0125 if (/san francisco/i).test city
  else
    tax = 0

  total = subtotal + shipping + tax
  $shipping.text util.humanizeNumber shipping.toFixed 2
  $tax.text util.humanizeNumber tax.toFixed 2
  $total.text util.humanizeNumber total.toFixed 2
  return

$state.change updateShippingAndTax
$city.on 'keyup', updateShippingAndTax
$country.change updateShippingAndTax

$(document).ready ->
  $form = $('#form')

  # Authorize with stripe
  stripeAuthorize = do ->
    app.set 'approved', false

    (status, response) ->
      console.log 'Got response from stripe', response
      if response.error
        $('#error-message').text response.error.message
      else
        app.set 'approved', true
        token = response.id
        $('input[name="StripeToken"]').val token
        $form.submit()
      return

  validateBilling = do ->
    $billingInfo = $('.billing-information')
    $billingInfo.find('input').change ->
      app.set 'validBillingAddress', false
    app.set 'validBillingAddress', false
    (err, exact, inexact) ->
      console.log 'Got response from google', arguments
      if !err?
        if exact? && exact.length > 0
          address = exact[0]
        else if inexact? && inexact.length > 0
          address = inexact[0]

        if address?
          alert = app.get 'alert'
          alert.show
            cover: true
            nextTo: $('.billing-information fieldset')
            title: 'Is this your street address?'
            message: address.toString()
            confirm: 'Yes'
            onConfirm: ->
              $billingInfo.find('#billing-address-1 input').val(address.streetNumber + ' ' + address.street)
              $billingInfo.find('#billing-city input').val(address.city)
              $billingInfo.find('#billing-state input').val(address.state)
              $billingInfo.find('#billing-zip input').val(address.postalCode)
              $billingInfo.find('#billing-country input').val(address.country)
              app.set 'validBillingAddress', true
              setTimeout ->
                $form.submit()
              , 10
            cancel:  'No'
            onCancel: ->
              $('#error-message').text 'We could not verify your billing address.  Please try again.'
          return true
      $billingInfo.find('input').addClass('error')
      $('#error-message').text 'We could not verify your billing address.  Please try again.'

  validateShipping = do ->
    $shippingInfo = $('.shipping-information')
    $shippingInfo.find('input').change ->
      app.set 'validShippingAddress', false
    app.set 'validShippingAddress', false
    (err, exact, inexact) ->
      console.log 'Got response from google', arguments
      if !err?
        if exact? && exact.length > 0
          address = exact[0]
        else if inexact? && inexact.length > 0
          address = inexact[0]

        if address?
          alert = app.get 'alert'
          alert.show
            cover: true
            nextTo: $('.shipping-information fieldset')
            title: 'Is this your street address?'
            message: address.toString()
            confirm: 'Yes'
            onConfirm: ->
              $shippingInfo.find('#shipping-address-1 input').val(address.streetNumber + ' ' + address.street)
              $shippingInfo.find('#shipping-city input').val(address.city)
              $shippingInfo.find('#shipping-state input').val(address.state)
              $shippingInfo.find('#shipping-zip input').val(address.postalCode)
              $shippingInfo.find('#shipping-country input').val(address.country)
              app.set 'validShippingAddress', true
              setTimeout ->
                $form.submit()
              , 10
            cancel:  'No'
            onCancel: ->
              $('#error-message').text 'We could not verify your shipping address.  Please try again.'
          return true
      $shippingInfo.find('input').addClass('error')
      $('#error-message').text 'We could not verify your shipping address.  Please try again.'

  # Create credit card fanciness: https://github.com/jessepollak/card
  $form.card
    container:   '#card-wrapper'
    numberInput: '#stripe-number'
    expiryInput: '#stripe-expiry-month, #stripe-expiry-year'
    cvcInput:    '#stripe-cvc'
    nameInput:   '#stripe-name'

    formatting: true

    values:
      number: '•••• •••• •••• ••••',
      name: 'Full Name',
      expiry: '••/••••',
      cvc: '•••'


  lock = false
  # Handle form submission
  $form.submit (e) ->
    # Do basic authorization
    unless validateForm()
      return false

    # unless app.get 'validBillingAddress'
    #   $billingInfo = $('.billing-information')
    #   address = new validator.Address
    #     street:     $billingInfo.find('#billing-address-1 input').val()
    #     city:       $billingInfo.find('#billing-city input').val()
    #     state:      $billingInfo.find('#billing-state input').val()
    #     postalCode: $billingInfo.find('#billing-zip input').val()
    #     country:    $billingInfo.find('#billing-country input').val()
    #   validator.validate(address, validator.match.streetAddress, validateBilling)
    #   location.href = "#billing-address"
    #   $('#error-message').text 'Please validate your billing address'
    #   return false

    # if !$('input[name="ShipToBilling"]').is(':checked') && !app.get('validShippingAddress')
    #   $shippingInfo = $('.shipping-information')
    #   address = new validator.Address
    #     street:     $shippingInfo.find('#shipping-address-1 input').val()
    #     city:       $shippingInfo.find('#shipping-city input').val()
    #     state:      $shippingInfo.find('#shipping-state input').val()
    #     postalCode: $shippingInfo.find('#shipping-zip input').val()
    #     country:    $shippingInfo.find('#shipping-country input').val()
    #   validator.validate(address, validator.match.streetAddress, validateShipping)
    #   location.href = "#shipping-address"
    #   $('#error-message').text 'Please validate your shipping address'
    #   return false

    # Do stripe authorization
    unless app.get 'approved'
      Stripe.card.createToken $form, stripeAuthorize
      return false

    if !lock
      lock = true
      $form.find('.btn-container button').append '<div class="loading-spinner" style="float:left"></div>'
      $.ajax
        url: $form.attr 'action'
        type: "POST"
        data: $form.serializeArray()
        success: ()->
          window.location.replace('complete/')
        error: ()->
          $('#error-message').text 'Sorry, we could not charge this card please try a different credit card.'
          $form.find('.loading-spinner').remove()
          lock = false

    # This should only happen when form is manually from `stripeAuthorize`
    false
