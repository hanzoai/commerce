View = require 'mvstar/lib/view'

class AlertView extends View
  el: '.sqs-widgets-confirmation.alert'

  constructor: (opts = {}) ->
    super
    @$nextTo       = $(opts.nextTo  ? 'body')
    @set 'confirm',   (opts.confirm ? 'okay')
    @set 'message',   (opts.message ? 'message')
    @set 'title',     (opts.title   ? 'title')

  bindings:
    title:   '.title'
    message: '.message'
    confirm: '.confirmation-button'

  events:
    # Dismiss on click, escape, and scroll
    'mousedown document': ->
      @dismiss()

    'keydown document': (e) ->
      e = event unless e
      @dismiss() if e.keyCode is 27

    'scroll window': ->
      @dismiss()

  # show alert box
  show: (opts = {}) ->
    (@set 'title',   opts.title)   if opts.title?
    (@set 'message', opts.message) if opts.message?
    (@set 'title',   opts.title)   if opts.title?

    @render()
    @bind()
    @position()
    @$el.fadeIn(200)

  # hide alert box
  dismiss: ->
    @unbind()
    @$el.fadeOut 200, => @$el.css top: -1000

  # update position relative to thing this should be next to
  position: ->
    offset = @$nextTo.offset()
    topOffset = offset.top - $(window).scrollTop()

    @$el.css
      position: 'fixed'
      top:      (topOffset   - 42) + 'px'
      left:     (offset.left - 66) + 'px'

module.exports = AlertView
