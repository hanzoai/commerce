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

