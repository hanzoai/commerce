util = require '../store/util'
require 'card'

# Validation helper
validation =
  isEmpty: (str) ->
    str.trim().length is 0

  isEmail: (email) ->
    pattern = new RegExp(/^[+a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,4}$/i)
    pattern.test email

validateForm = ->
  $('#error-message').text ''
  console.log 'validating form'

  valid = true
  errors = []

  # Get all inputs that are visible and empty
  empty = $('div:visible.required > input').filter ->
    validation.isEmpty $(@).val()

  window.empty = empty

  email = $('input[name="User.Email"]')
  unless validation.isEmail email.val()
    valid = false
    email.parent().addClass 'error'
    email.parent().addClass 'shake'
    setTimeout ->
      email.parent().removeClass 'shake'
      return
    , 500
    $('#error-message').text "Please fill out: #{}"

  if empty.length > 0
    valid = false
    empty.parent().addClass 'error'
    empty.parent().addClass 'shake'
    setTimeout ->
      empty.parent().removeClass 'shake'
      return
    , 500

  unless valid
    # display errors
    labels = (label.trim() for label in empty.parent().text().split('\n') when label.trim())
    $('#error-message').text "Please check: #{labels.join ', '}."

    # scroll to first error
    location.href = '#' + empty[0].id
    pos = $(window).scrollTop() - 100 # save position
    location.hash = '' # clear hash
    $(window).scrollTop pos

  return valid

# Remove error class from field that has been edited or clicked on
clearError = -> $(@).removeClass 'error'
$('div.field').on 'click', clearError
$('div.field').on 'change', clearError


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

  # Handle form submission
  $form.submit (e) ->
    # Do basic authorization
    unless validateForm()
      return false

    # Do stripe authorization
    unless app.get 'approved'
      Stripe.card.createToken $form, stripeAuthorize
      return false

    # This should only happen when form is manually from `stripeAuthorize`
    true
