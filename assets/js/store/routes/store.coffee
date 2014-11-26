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
