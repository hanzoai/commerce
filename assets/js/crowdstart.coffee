# Helper functions
humanizeNumber = (num) ->
  num.toString().replace /(\d)(?=(\d\d\d)+(?!\d))/g, "$1,"

formatCurrency = (num) ->
  currency = num or 0
  humanizeNumber currency.toFixed(2)

window.csio = window.csio or {}
csio.cookieName = "SKULLYSystemsCart"

$.cookie.json = true
csio.Alert = (opts) ->

  # Show

  # Dismiss
  dismiss = ->
    $el.fadeOut 200, ->
      $el.css top: -1000
      return

    return
  $el = $(".sqs-widgets-confirmation.alert")
  offset = opts.$nextTo.offset()
  topOffset = offset.top - $(window).scrollTop()
  $el.find(".title").text opts.title
  $el.find(".message").text opts.message
  $el.find(".confirmation-button").text opts.confirm
  $el.css
    position: "fixed"
    top: (topOffset - 42) + "px"
    left: (offset.left - 66) + "px"

  $el.fadeIn 200

  # Dismiss on click, escape, and scroll
  $(document).mousedown ->
    dismiss()
    return

  $(document).keydown (e) ->
    e = event  unless e
    dismiss()  if e.keyCode is 27
    return

  $(window).scroll ->
    dismiss()
    return

  return


# Lookup variant based on selected options.
csio.getVariant = ->

  # Determine if selected options match variant
  optionsMatch = (selected, variant) ->
    for k of selected
      continue
    true
  selected = {}
  variants = csio.currentProduct.Variants
  missingOptions = []

  # Get selected options
  $(".variant-option").each (i, v) ->
    $(v).find("select").each (i, v) ->
      $select = $(v)
      name = $select.data("variant-option-name")
      value = $select.val()
      selected[name] = value
      missingOptions.push name  if value is "none"
      return

    return


  # Warn if missing options (we'll be unable to figure out a SKU).
  if missingOptions.length > 0
    return csio.Alert(
      title: "Unable To Add Item"
      message: "Please select a " + missingOptions[0] + " option."
      confirm: "Okay"
      $nextTo: $(".sqs-add-to-cart-button")
    )

  # Figure out SKU
  i = 0

  while i < variants.length
    variant = variants[i]

    # All options match match variant
    return variant  if optionsMatch(selected, variant)
    i++

  # Only one variant, no options.
  variants[0]


# Add to cart
csio.addToCart = ->
  quantity = parseInt($("#quantity").val(), 10)
  cart = $.cookie(csio.cookieName) or {}
  variant = csio.getVariant()
  return  unless variant?
  sku = variant.sku
  if cart[sku]
    cart[sku].quantity += quantity
  else
    cart[sku] =
      sku: variant.SKU
      color: variant.Color
      img: csio.currentProduct.Images[0].Url
      name: csio.currentProduct.Title
      quantity: quantity
      size: variant.Size
      price: parseInt(variant.Price, 10) * 0.0001
      slug: csio.currentProduct.Slug

  # Set cookie
  csio.setCart cart
  inner = $(".sqs-add-to-cart-button-inner")
  inner.html ""
  inner.append "<div class=\"yui3-widget sqs-spin light\" ></div>"
  inner.append "<div class=\"status-text\">Adding...</div>"
  setTimeout (->
    $(".status-text").text("Added!").fadeOut 500, ->
      inner.html "Add to Cart"
      return

    return
  ), 500
  setTimeout (->

    # Flash cart hover
    $(".sqs-pill-shopping-cart-content").animate
      opacity: 0.85
    , 400, ->

      # Update cart hover
      csio.updateCartHover cart
      $(".sqs-pill-shopping-cart-content").animate
        opacity: 1
      , 300
      return

    return
  ), 300
  return

csio.setCart = (cart) ->
  $.cookie csio.cookieName, cart,
    expires: 30
    path: "/"

  return

csio.getCart = ->
  $.cookie(csio.cookieName) or {}

csio.clearCart = ->
  $.cookie csio.cookieName, {},
    expires: 30
    path: "/"

  return

csio.updateCartHover = (modifiedCart) ->
  cart = modifiedCart or csio.getCart()
  numItems = 0
  subTotal = 0
  for k of cart
    lineItem = cart[k]
    numItems += lineItem.quantity
    subTotal += lineItem.price * lineItem.quantity
  $(".total-quantity").text humanizeNumber(numItems)
  $(".subtotal .price span").text formatCurrency(subTotal)
  if numItems is 1
    $(".details span.suffix").text "item"
  else
    $(".details span.suffix").text "items"
  return


# Events

# Show cart when cart button is clicked
$(".fixed-cart").click ->
  window.location = "/cart"
  return


# Product gallery image switching
$("#productThumbnails .slide img").each (i, v) ->
  $(v).click ->
    src = $(v).data("src")
    $("#productSlideshow .slide img").each (i, v) ->
      if src is $(v).data("src")
        $(v).fadeIn 400
      else
        $(v).fadeOut 400
      return

    return

  return


# Update cart hover onload
csio.updateCartHover()

# PAGE SPECIFIC HACKS

# Hide cart button on cart page
$(".fixed-cart").hide()  if location.pathname is "/cart"

# Swap images when changing colors on helmet page
if location.pathname is "/products/ar-1"
  $slides = $("#productSlideshow .slide img")
  $("[data-variant-option-name=Color]").change ->
    if $(this).val() is "Black"
      $($slides[0]).fadeIn()
      $($slides[1]).fadeOut()
    else
      $($slides[1]).fadeIn()
      $($slides[0]).fadeOut()
    return

csio.NumbersOnly = (event) ->
  event.charCode >= 48 and event.charCode <= 57

require './cart'
require './checkout'
