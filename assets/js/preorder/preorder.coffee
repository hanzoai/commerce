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
    routes.order.displayPerks
    routes.order.displayHelmets
  ]
  '*': [
    (-> console.log 'global')
  ]

app.start()

window.helmetTotal = helmetTotal = 0
window.gearTotal = gearTotal = 0

$(document).ready ->

  # Total from perks
  countFunc = (selector, total) ->
    ->

      # Start looping over everything else
      itemEl = $(selector)
      counterEl = itemEl.find(".counter")
      count = 0

      itemEl.find(".form:first .quantity").each ->
        val = parseInt($(this).val(), 10)
        val = 0  if isNaN(val)
        count += val
        return

      unless count is total
        counterEl.addClass "bad"
      else
        counterEl.removeClass "bad"
      counterEl.html count
      itemEl.find(".total").html "/" + total + ")"
      return

  appendFunc = (selector, variantT, countF) ->
    count = 0
    append = ->
      variantEl = $(variantT + " .form:first")
      if count > 0
        subButtonEl = $(subButtonT)
        subButtonEl.on "click", ->
          variantEl.remove()
          count--
          countF()
          return

        variantEl.append subButtonEl
      variantEl.find("input#quantity").payment("restrictNumeric").on "change keyup keypress", countF
      variantEl.find("button.add").on "click", append

      # Start here
      # variantEl.find('#color').attr
      $(selector).find(".form:first").append variantEl
      count++
      false

    append

  setText = (el, selector, data) ->
    el.find(selector).text data
    return

  # AR 1 stuff, refactor
  setValue = (selector, data) ->
    $(selector).val data  unless data is ""
    return

  validateCount = ->
    ar1Count = parseInt($(".item.ar1 .counter").text(), 10)
    apparelCount = parseInt($(".item.apparel .counter").text(), 10)
    ret = true
    unless ar1Count is helmetTotal
      $(".item.ar1 .quantity").addClass "fix"
      ret = false
    unless apparelCount is gearTotal
      $(".item.apparel .quantity").addClass "fix"
      ret = false
    ret

  subButtonT = "<button class=\"sub\">-</button>"
  ar1VariantT = "<div class=\"row variant\">  <select id=\"color\" name=\"HelmetColor\" class=\"color\">    <option value=\"Matte Black\">Matte Black</option>    <option value=\"Gloss White\">Gloss White</option>  </select>  <select id=\"size\" name=\"HelmetSize\" class=\"size\">    <option value=\"S\">S</option>    <option value=\"M\">M</option>    <option value=\"L\">L</option>    <option value=\"XL\">XL</option>    <option value=\"XXL\">XXL</option>  </select>  <input id=\"quantity\" class=\"quantity\" name=\"HelmetQuantity\" type=\"text\" maxlength=\"2\" placeholder=\"Qty.\">  <button class=\"add\">+</button></div>"
  apparelVariantT = "<div class=\"row variant\">  <select id=\"type\" name=\"ShirtStyle\" class=\"type\">    <option value=\"Men's Shirt\">Men's Shirt</option>    <option value=\"Women's Shirt\">Women's Shirt</option>  </select>  <select id=\"color\" name=\"ShirtColor\" class=\"color\">    <option value=\"Matte Black\">Matte Black</option>    <option value=\"Shinny Black\">Shiny Black</option>    <option value=\"Glossy Black\">Glossy Black</option>    <option value=\"Dark Black\">Dark Black</option>    <option value=\"Super Black\">Super Black</option>  </select>  <select id=\"size\" name=\"ShirtSize\" class=\"size\">    <option value=\"S\">S</option>    <option value=\"M\">M</option>    <option value=\"L\">L</option>    <option value=\"XL\">XL</option>  </select>  <input id=\"quantity\" name=\"ShirtQuantity\" class=\"quantity\" type=\"text\" maxlength=\"2\" placeholder=\"Qty.\">  <button class=\"add\">+</button></div>"

  countAr1 = countFunc(".item.ar1", helmetTotal)
  appendAr1 = appendFunc(".item.ar1", ar1VariantT, countAr1)
  appendAr1()
  countAr1()

  countApparel = countFunc(".item.apparel", gearTotal)
  appendApparel = appendFunc(".item.apparel", apparelVariantT, countApparel)
  appendApparel()
  countApparel()

  setValue "#email", PreorderData.user.Email
  setValue "#first_name", PreorderData.user.FirstName
  setValue "#last_name", PreorderData.user.LastName
  setValue "#phone", PreorderData.user.Phone
  setValue "#address1", PreorderData.user.ShippingAddress.Line1
  setValue "#address2", PreorderData.user.ShippingAddress.Line2
  setValue "#city", PreorderData.user.ShippingAddress.City
  setValue "#state", PreorderData.user.ShippingAddress.State
  setValue "#postal_code", PreorderData.user.ShippingAddress.PostalCode

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
