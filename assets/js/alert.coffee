module.exports = (opts) ->
  # Show

  # Dismiss
  dismiss = ->
    $el.fadeOut 200, ->
      $el.css top: -1000

  $el = $(".sqs-widgets-confirmation.alert")
  offset = opts.$nextTo.offset()
  topOffset = offset.top - $(window).scrollTop()

  $el.find(".title").text opts.title
  $el.find(".message").text opts.message
  $el.find(".confirmation-button").text opts.confirm

  $el.css
    position: "fixed"
    top: (topOffset - 42) + "px"
    left: (offset.left - 66) + "px"

  $el.fadeIn 200

  # Dismiss on click, escape, and scroll
  $(document).mousedown ->
    dismiss()
    return

  $(document).keydown (e) ->
    e = event  unless e
    dismiss()  if e.keyCode is 27
    return

  $(window).scroll ->
    dismiss()
