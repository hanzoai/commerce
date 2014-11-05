Alert = require '../views/alert'

exports.alert = ->
  app.alert = new Alert()

# Product gallery image switching
exports.gallery = ->
  $images     = ($(i) for i in $('#productSlideshow .slide img'))
  $thumbnails = ($(i) for i in $('#productThumbnails .slide img'))

  for $thumb in $thumbnails
    $thumb.click ->
      src = $thumb.data('src')
      for $img in $images
        if src is $img.data 'src'
          $img.fadeIn 400
        else
          $img.fadeOut 400

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
