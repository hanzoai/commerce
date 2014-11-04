# Product gallery image switching
exports.gallery = ->
  for thumb in $('#productThumbnails .slide img')
    thumb.click ->
      src = img.data('src')
      for img in $('#productSlideshow .slide img')
        if src is img.data 'src'
          img.fadeIn 400
        else
          img.fadeOut 400

exports.customizeAr1 = ->
    $slides = $('#productSlideshow .slide img')
    $('[data-variant-option-name=Color]').change ->
      if $(this).val() is "Black"
        $slides[0].fadeIn()
        $slides[1].fadeOut()
      else
        $slides[1].fadeIn()
        $slides[0].fadeOut()
