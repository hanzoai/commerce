exports.click = ->
  $('.fixed-cart').click ->
    window.location = '/cart'

exports.hideHover = ->
  $(".fixed-cart").hide()

exports.setupHover = ->
  view = new (require '../views/cart-hover')
  app.views.push view
  view.update()

exports.setupView = ->
