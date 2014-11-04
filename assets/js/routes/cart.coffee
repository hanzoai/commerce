cart = require '../cart'

exports.click = ->
  $('.fixed-cart').click ->
    window.location = '/cart'

exports.hideHover = ->
  $(".fixed-cart").hide()

exports.updateHover = ->
  cart.updateHover()

