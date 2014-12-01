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

$("div.field").on "click", ->
  $(this).removeClass "error"
  return

$("#form").submit (e) ->
  empty = $("div:visible.required > input").filter(->
    $(this).val() is ""
  )
  email = $("input[name=\"User.Email\"]")
  unless validation.isEmail(email.val())
    console.log validation.isEmail(email.text())
    e.preventDefault()
    email.parent().addClass "error"
    email.parent().addClass "shake"
    setTimeout (->
      email.parent().removeClass "shake"
      return
    ), 500
  if empty.length > 0
    e.preventDefault()
    empty.parent().addClass "error"
    empty.parent().addClass "shake"
    setTimeout (->
      empty.parent().removeClass "shake"
      return
    ), 500
  return

# Show payment options when first half is competed.
$requiredVisible = $("div:visible.required > input")
showPaymentOptions = $.debounce(250, ->

  # Check if all required inputs are filled
  i = 0

  while i < $requiredVisible.length
    return  if $requiredVisible[i].value is ""
    i++
  fieldset = $("div.payment-information > fieldset")
  fieldset.css "display", "block"
  fieldset.css "opacity", "0"
  fieldset.fadeTo 1000, 1
  $requiredVisible.off "keyup", showPaymentOptions
  return
)
$requiredVisible.on "keyup", showPaymentOptions
$("#form").card
  container: "#card-wrapper"
  numberInput: "#stripe-number"
  expiryInput: "#stripe-expiry-month, #stripe-expiry-year"
  cvcInput: "#stripe-cvc"
  nameInput: "#stripe-name"

$("input[name=\"ShipToBilling\"]").change ->
  shipping = $(".shipping-information fieldset")
  if @checked
    shipping.fadeOut 500
    setTimeout (->
      shipping.css "display", "none"
      return
    ), 500
  else
    shipping.fadeIn 500
    shipping.css "display", "block"
  return


# Update tax display
$state = $("input[name=\"Order.BillingAddress.State\"]")
$city = $("input[name=\"Order.BillingAddress.City\"]")
$tax = $("div.tax .price")
$total = $("div.grand-total .price")
$subtotal = $("div.subtotal .price")

updateTax = $.debounce 250, ->
  city = $city.val()
  state = $state.val().toUpperCase()
  tax = 0
  total = 0
  subtotal = parseFloat($subtotal.text().replace(",", ""))

  # Add CA tax
  tax += subtotal * 0.075  if state is "CA" or (/california/i).test(state)

  # Add SF county tax
  tax += subtotal * 0.0125  if state is "CA" and (/san francisco/i).test(city)
  total = subtotal + tax
  $tax.text tax.toFixed(2)
  $total.text total.toFixed(2)
  return

$state.change updateTax
$city.on "keyup", updateTax


$form = $("form#stripeForm")
$cardNumber = $("#stripe-number")
$expiryMonth = $("#stripe-expiry-month")
$expiryYear = $("#stripe-expiry-year")
$cvc = $("#stripe-cvc")
$token = $("input[name=\"StripeToken\"]")

# Checks each input and does dumb checks to see if it might be a valid card
validateCard = ->
  fail = success: false
  cardNumber = $cardNumber.val()
  return fail  if cardNumber.length < 10
  month = $expiryMonth.val()
  year = $expiryYear.val()

  return fail unless month.length is 2
  return fail unless year.length is 4
  cvc = $cvc.val()

  return fail  if cvc.length < 3

  success: true
  number: cardNumber
  month: month
  year: year
  cvc: cvc

$authorizeMessage = $("#authorize-message")

# Callback for createToken
stripeResponseHandler = do ->
  app.set 'approved', false
  (status, response) ->
    $authorizeMessage.removeClass "error"
    if response.error
      $authorizeMessage.text response.error.message
      $authorizeMessage.addClass "error"
    else
      app.set 'approved', true
      token = response.id
      $token.val token
      $authorizeMessage.text "Card approved. Ready when you are."
    return

# Copies validated card values into the hidden form for Stripe.js
stripeRunner = ->
  card = validateCard()
  Stripe.card.createToken $form, stripeResponseHandler if card.success
  return

relayer = ->
  card = validateCard()
  if card.success
    $form.find('input[data-stripe="number"]').val card.number
    $form.find('input[data-stripe="cvc"]').val card.cvc
    $form.find('input[data-stripe="exp-month"]').val card.month
    $form.find('input[data-stripe="exp-year"]').val card.year
  return

$cardNumber.change relayer
$expiryMonth.change relayer
$expiryYear.change relayer
$cvc.change relayer

$(document).ready ->
  $("#form").submit (event) ->
    unless app.get('approved')
      stripeRunner()
      return false
    true
  return
