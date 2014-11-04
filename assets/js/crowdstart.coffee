alert = require './alert'
util = require './util'

# Globals
window.csio = window.csio or {}
csio.cookieName = "SKULLYSystemsCart"
$.cookie.json = true

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
