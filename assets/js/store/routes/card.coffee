toBlob = require('blueimp-canvas-to-blob/js/canvas-to-blob') # canvas.toBlob polyfill

ratio       = 0.4
$skullyCard = $('#skully-card')
$giftCard   = $('#gift-card')

renderImgToCanvas = (img, canvas) ->
  canvas.width = img.width * ratio
  canvas.height = img.height * ratio
  ctx = canvas.getContext('2d')
  ctx.scale ratio, ratio
  ctx.drawImage img, 0, 0
  return

getFontSize = (text, ctx) ->
  # Draw Text
  fontSize = 300
  ctx.font = 'bold ' + fontSize + 'px Michroma'
  ctx.fillStyle = '#FFF'

  # Max width of text
  maxWidth = 1980

  # May not work in old IE
  try
    width = ctx.measureText(text).width
    while width > maxWidth
      fontSize -= 0.5
      ctx.font = 'bold ' + fontSize + 'px Michroma'
      width = ctx.measureText(text).width
  fontSize

renderSkullyCard = (img, canvas) ->
  name = window.userName.toUpperCase()
  ctx = canvas.getContext('2d')
  ctx.scale ratio, ratio
  renderImgToCanvas img, canvas
  fontSize = getFontSize(name, ctx)
  ctx.fillText name, 520, 1115 + fontSize * 0.8
  canvas.toDataURL()

renderGiftCard = (img, canvas) ->
  fromName = window.userName.toUpperCase()
  toName = window.toName.toUpperCase()
  ctx = canvas.getContext('2d')
  ctx.scale ratio, ratio
  renderImgToCanvas img, canvas
  fontSize = getFontSize(fromName, ctx)
  ctx.fillText fromName, 520, 680 + fontSize * 0.8
  fontSize = getFontSize(toName, ctx)
  ctx.fillText toName, 520, 1115 + fontSize * 0.8
  canvas.toDataURL()

setActiveCard = (showGift) ->
  if showGift
    hideImg = $skullyCard
    showImg = $giftCard
    $('.recipient').removeClass 'hidden'
  else
    hideImg = $giftCard
    showImg = $skullyCard
    $('.recipient').addClass 'hidden'

  spinner = $('.loading-spinner')

  hideImg.addClass 'hidden'
  spinner.removeClass 'hidden'
  setTimeout ->
    hideImg.addClass 'none'
    spinner.addClass 'hidden'
    showImg.removeClass 'none'
    showImg.removeClass 'hidden'
  , 301

exports.renderCards = ->
  showGift = false
  WebFont.load
    google:
      families: ["Michroma"]

    active: ->
      canvas = $('<canvas>')[0]
      # Half the size of the rendering
      img1 = $('<img>').attr 'src', window.cardName
      img2 = $('<img>').attr 'src', window.giftCardName
      imgBack = $('<img>').attr 'src', window.cardBack

      img1.load ->
        $skullyCard.attr('src', renderSkullyCard(img1[0], canvas)).removeClass('hidden').removeClass 'none'
        $('.placeholder').addClass 'none'
        $('.loading-spinner').addClass 'hidden'

      img2.load ->
        $giftCard.attr 'src', renderGiftCard(img2[0], canvas)
        $('.placeholder').addClass 'none'
        $('.loading-spinner').addClass 'hidden'

      $('.is-gift input[type=radio]').click ->
        setActiveCard(showGift = $(@).val() == 'Yes')

      recipientId = 0
      $('input[name="Recipient"]').keydown ->
        spinner = $('.loading-spinner')
        spinner.removeClass 'hidden'

        clearTimeout(recipientId)
        recipientId = setTimeout =>
          spinner.addClass 'hidden'
          window.toName = $(@).val()
          $('#GiftCard').attr 'src', renderGiftCard(img2[0], canvas)
        , 300

      $('.download').click ->
        link = $('<a>')
        link.attr 'download', 'skullycard.png'
        img = (if showGift then $giftCard else $skullyCard)[0]

        # render the downloadable image with card back
        width = img1[0].width * ratio
        height = img1[0].height * ratio

        bufferCanvas = $('<canvas>')[0]
        bufferCanvas.width = width
        bufferCanvas.height = height * 2

        ctx = bufferCanvas.getContext('2d')
        ctx.drawImage img, 0, height
        ctx.scale ratio, ratio
        ctx.translate width / ratio, height / ratio
        ctx.rotate Math.PI
        ctx.drawImage imgBack[0], 0, 0

        link.attr 'href', bufferCanvas.toDataURL()
        link[0].click()

      $('.share').click ->
        img = (if showGift then $giftCard else $skullyCard)[0]

        # render the downloadable image with card back
        width = img1[0].width * ratio
        height = img1[0].height * ratio

        bufferCanvas = $('<canvas>')[0]
        bufferCanvas.width = width
        bufferCanvas.height = height * 2

        ctx = bufferCanvas.getContext('2d')
        ctx.drawImage img, 0, height
        ctx.scale ratio, ratio
        ctx.translate width / ratio, height / ratio
        ctx.rotate Math.PI
        ctx.drawImage imgBack[0], 0, 0

        bufferCanvas.toBlob (blob) ->
          filename = "skully-xmas-card/#{Math.random().toString(36).slice(2)}/skully-xmas-card.png"

          $.ajax
            method: 'POST'
            url: "https://www.googleapis.com/upload/storage/v1/b/#{GCS_BUCKET}/o?uploadType=media&name=#{filename}&predefinedAcl=publicRead&key=#{GCS_API_KEY}"
            processData: false
            contentType: false
            data: blob
            headers:
              'Content-Type': "image/png"
              'Content-Length': blob.size
            success: ->
              console.log arguments
              $('.share-link').val "https://storage.googleapis.com/#{GCS_BUCKET}/#{filename}"
              $('.share-options').fadeIn()
              $('.share-link').click ->
                $(@).select()

            error: ->
              console.log arguments
