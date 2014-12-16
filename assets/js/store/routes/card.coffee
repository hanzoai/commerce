renderImgToCanvas = (img, canvas) ->
  canvas.width = img.width
  canvas.height = img.height
  ctx = canvas.getContext("2d")
  ctx.drawImage img, 0, 0
  return

getFontSize = (text, ctx) ->
  # Draw Text
  fontSize = 300
  ctx.font = "bold " + fontSize + "px Michroma"
  ctx.fillStyle = "#FFF"

  # Max width of text
  maxWidth = 1980

  # May not work in old IE
  try
    width = ctx.measureText(text).width
    while width > maxWidth
      fontSize -= 0.5
      ctx.font = "bold " + fontSize + "px Michroma"
      width = ctx.measureText(text).width
  fontSize

renderSkullyCard = (img, canvas) ->
  name = window.userName.toUpperCase()
  ctx = canvas.getContext("2d")
  renderImgToCanvas img, canvas
  fontSize = getFontSize(name, ctx)
  ctx.fillText name, 520, 1115 + fontSize * 0.8
  canvas.toDataURL()

renderGiftCard = (img, canvas) ->
  fromName = window.userName.toUpperCase()
  toName = window.toName.toUpperCase()
  ctx = canvas.getContext("2d")
  renderImgToCanvas img, canvas
  fontSize = getFontSize(fromName, ctx)
  ctx.fillText fromName, 520, 680 + fontSize * 0.8
  fontSize = getFontSize(toName, ctx)
  ctx.fillText toName, 520, 1115 + fontSize * 0.8
  canvas.toDataURL()

setActiveCard = (showGift) ->
  if showGift
    hideImg = $('#SkullyCard')
    showImg = $('#GiftCard')
    $('.recipient').show()
  else
    hideImg = $('#GiftCard')
    showImg = $('#SkullyCard')
    $('.recipient').hide()

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
      img1 = $('<img>').attr 'src', window.cardName
      img2 = $('<img>').attr 'src', window.giftCardName
      img1.load ->
        $('#SkullyCard').attr('src', renderSkullyCard(img1[0], canvas)).removeClass('hidden').removeClass 'none'
        $('.placeholder').addClass 'none'
        $('.loading-spinner').addClass 'hidden'

      img2.load ->
        $('#GiftCard').attr 'src', renderGiftCard(img2[0], canvas)
        $('.placeholder').addClass 'none'
        $('.loading-spinner').addClass 'hidden'

      $('.is-gift input[type=radio]').click ->
        setActiveCard(showGift = $(@).val() == 'Yes')

      recipientId = 0
      $('input[name="Recipient"').keydown ->
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
        link.attr 'href', if showGift then $('#GiftCard').attr('src') else $('#SkullyCard').attr('src')
        link[0].click()



