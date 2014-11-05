ProductView = require '../views/product'

# setup view
exports.setupView = ->
  view = new ProductView()
  app.views.push view
  view.bind()

# Product gallery image switching
exports.gallery = ->
  $('#productThumbnails .slide img').each (i, v) ->
    $(v).click ->
      src = $(v).data('src')
      $('#productSlideshow .slide img').each (i, v) ->
        if src is $(v).data('src')
          $(v).fadeIn 400
        else
          $(v).fadeOut 400

# Swap AR-1 helmets when color selected
exports.customizeAr1 = ->
  $slides = ($(i) for i in $('#productSlideshow .slide img'))

  $('[data-variant-option-name=Color]').change ->
    if $(@).val() is "Black"
      $slides[0].fadeIn()
      $slides[1].fadeOut()
    else
      $slides[1].fadeIn()
      $slides[0].fadeOut()
