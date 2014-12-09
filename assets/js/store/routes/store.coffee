ProductView = require '../views/product'

exports.setupViews = ->
  console.log 'store#setupViews'
  console.log 'hi'
  for div in $('.product-text')
    do (div) ->
      console.log 'product'
      view = new ProductView el: $(div)
      window.view = view
      app.views.push view
      view.bind()
      view.render()

# Simple thumbnail gallery
exports.gallery = ->
  fading = false
  $(document).ready ->
    $('.product-viewer .gallery .thumbnail').on 'click', ->
      if !fading
        fading = true
        thumbnail = $(@)
        gallery = thumbnail.parent()
        viewer = gallery.parent()
        preview = viewer.find '.preview-background'
        overlay = viewer.find '.preview'

        src = thumbnail.attr 'src'
        alt = thumbnail.attr 'alt'

        gallery.children().removeClass 'selected'
        thumbnail.addClass 'selected'

        preview.css
          "background-image": 'url(' + overlay.attr('src') + ')'
        overlay.css
          opacity: 0
        overlay.attr
          src: src
        overlay.animate
          opacity: 1
          , 300, 'swing', ->
            fading = false
            preview.css
              "background-image": 'url(' + src + ')'

exports.setupStylesAndSizes =->
  $(document).ready ->
    $('.size').val('M')
    $('.style').on 'change', ->
      style = $(@)
      config = style.parent()

      size = config.find('.size')

      if style.val() == "Men's T-Shirt"
        hasOptions = false
        size.find('option').each ->
          option = $(@)
          hasOptions = true if option.val() == 'XXL' || option.val() == 'XXXL'
        if !hasOptions
          size.append $('<option value="XXL">XXL</option>')
          size.append $('<option value="XXXL">XXXL</option>')
      else if style.val() =="Women's T-Shirt"
        size.find('option').each ->
          option = $(@)
          option.remove() if option.val() == 'XXL' || option.val() == 'XXXL'

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

exports.menu = ->
  $('.menu-icon').click ->
    $('body').toggleClass('mobile')
