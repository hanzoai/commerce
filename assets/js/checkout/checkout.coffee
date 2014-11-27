# View = require './view'

# class CardView extends View
#   el: '.sqs-checkout-form-payment'

# Globals
window.csio = window.csio or {}

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
  fieldset = $("div.sqs-checkout-form-payment-content > fieldset")
  fieldset.css "display", "block"
  fieldset.css "opacity", "0"
  fieldset.fadeTo 1000, 1
  $requiredVisible.off "keyup", showPaymentOptions
  return
)
$requiredVisible.on "keyup", showPaymentOptions
$("#form").card
  container: "#card-wrapper"
  numberInput: "#cardNumber"
  expiryInput: "#expiry"
  cvcInput: "#cvcInput"
  nameInput: "input[name=\"User.FirstName\"], input[name=\"User.LastName\"]"

$("input[name=\"ShipToBilling\"]").change ->
  shipping = $("#shippingInfo")
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
$state = $("select[name=\"Order.BillingAddress.State\"]")
$city = $("input[name=\"Order.BillingAddress.City\"]")
$tax = $("div.tax.total > div.price > span")
$total = $("div.grand-total.total > div.price > span")
$subtotal = $("div.subtotal.total > div.price > span")

updateTax = $.debounce 250, ->
  city = $city.val()
  state = $state.val()
  tax = 0
  total = 0
  subtotal = parseFloat($subtotal.text().replace(",", ""))

  # Add CA tax
  tax += subtotal * 0.075  if state is "CA"

  # Add SF county tax
  tax += subtotal * 0.0125  if state is "CA" and (/san francisco/i).test(city)
  total = subtotal + tax
  $tax.text tax.toFixed(2)
  $total.text total.toFixed(2)
  return

$state.change updateTax
$city.on "keyup", updateTax
