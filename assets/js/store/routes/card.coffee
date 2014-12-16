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

exports.renderCards = ->
  WebFont.load
    google:
      families: ["Michroma"]

    active: ->
      canvas = document.createElement("canvas")
      img1 = $("<img>").attr("src", window.cardName)
      img2 = $("<img>").attr("src", window.giftCardName)
      img1.load ->
        $("#SkullyCard").attr("src", renderSkullyCard(img1[0], canvas)).removeClass("hidden").removeClass "none"
        $(".placeholder").addClass "none"
        $(".loading-spinner").addClass "hidden"

      img2.load ->
        $("#GiftCard").attr "src", renderGiftCard(img2[0], canvas)
        $(".placeholder").addClass "none"
        $(".loading-spinner").addClass "hidden"

