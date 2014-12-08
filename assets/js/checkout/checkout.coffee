util = require '../store/util'

require 'card'
# View = require './view'

# class CardView extends View
#   el: '.sqs-checkout-form-payment'

# Validation helper
validation =
  isEmpty: (str) ->
    str.trim().length is 0

  isEmail: (email) ->
    pattern = new RegExp(/^[+a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,4}$/i)
    pattern.test email

$('div.field').on 'click', ->
  $(this).removeClass 'error'
  return

$('#form').submit (e) ->
  empty = $('div:visible.required > input').filter ->
    $(this).val() is ''

  email = $('input[name="User.Email"]')
  unless validation.isEmail email.val()
    console.log validation.isEmail email.text()
    e.preventDefault()
    email.parent().addClass 'error'
    email.parent().addClass 'shake'
    setTimeout (->
      email.parent().removeClass 'shake'
      return
    ), 500
  if empty.length > 0
    e.preventDefault()
    empty.parent().addClass 'error'
    empty.parent().addClass 'shake'
    setTimeout (->
      empty.parent().removeClass 'shake'
      return
    ), 500
  return

$('#form').card
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

$stripeForm = $('form#stripeForm')
$cardNumber = $('#stripe-number')
$expiryMonth = $('#stripe-expiry-month')
$expiryYear = $('#stripe-expiry-year')
$cvc = $('#stripe-cvc')
$token = $('input[name="StripeToken"]')

$authorizeMessage = $('#authorize-message')

# Callback for createToken
stripeResponseHandler = do ->
  app.set 'approved', false
  (status, response) ->
    $authorizeMessage.removeClass 'error'
    if response.error
      $authorizeMessage.text response.error.message
      $authorizeMessage.addClass 'error'
    else
      app.set 'approved', true
      token = response.id
      $token.val token
      $('#form').submit()
    return

updateStripeForm = ->
  $stripeForm.find('input[data-stripe="number"]').val card.number
  $stripeForm.find('input[data-stripe="cvc"]').val card.cvc
  $stripeForm.find('input[data-stripe="exp-month"]').val card.month
  $stripeForm.find('input[data-stripe="exp-year"]').val card.year

$cardNumber.change updateStripeForm
$expiryMonth.change updateStripeForm
$expiryYear.change updateStripeForm
$cvc.change updateStripeForm

$(document).ready ->
  $('#form').submit (event) ->
    unless app.get('approved')
      Stripe.card.createToken $form, stripeResponseHandler
      return false
    true
  return
