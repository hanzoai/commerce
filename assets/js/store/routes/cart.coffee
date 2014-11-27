exports.click = ->
  $('.cart-hover').click ->
    window.location = '/cart'

exports.hideHover = ->
  $(".cart-hover").hide()

exports.setupHover = ->
  view = new (require '../views/cart-hover')
  app.views.push view
  view.listen()

exports.setupView = ->
  view = new (require '../views/cart')
  app.views.push view
  view.render()
