View = require 'mvstar/lib/view'

class AlertView extends View
  el: '.alert-dialog'

  constructor: (opts = {}) ->
    super
    @$nextTo =         $(opts.nextTo      ? 'body')
    @set 'confirm',     (opts.confirm     ? 'okay')
    @set 'cancel',      (opts.cancel      ? 'cancel')
    @set 'message',     (opts.message     ? 'message')
    @set 'title',       (opts.title       ? 'title')
    @set 'autodismiss', (opts.autodismiss ? false)

    @cover   =          false

  bindings:
    title:   '.alert-title'
    message: '.alert-message'
    confirm: '.alert-confirmation-button'
    cancel:  '.alert-cancel-button'

  events:
    # Dismiss on click, escape, and scroll
    'mousedown document': ->
      @dismiss() if (@get 'autodismiss')

    'keydown document': (e) ->
      return unless (get 'autodismiss')
      e = event unless e
      @dismiss() if e.keyCode is 27

    'scroll window': ->
      @dismiss() if (@get 'autodismiss')

    'click .alert-confirmation-button': ->
      @dismiss()
      @onConfirm() if @onConfirm

    'click .alert-cancel-button': ->
      @dismiss()
      @onCancel() if @onCancel

  # show alert box
  show: (opts = {}) ->
    (@set 'title',       opts.title)       if opts.title?
    (@set 'message',     opts.message)     if opts.message?
    (@set 'confirm',     opts.confirm)     if opts.confirm?
    (@set 'cancel',      opts.cancel)      if opts.cancel?
    (@set 'autodismiss', opts.autodismiss) if opts.autodismiss?

    @$nextTo   = opts.nextTo if opts.nextTo?
    @cover     = opts.cover  if opts.cover?
    @onConfirm = opts.onConfirm if opts.onConfirm
    @onCancel  = opts.onCancel  if opts.onCancel

    @render()
    @bind()
    @position()
    @$el.finish().fadeIn(200)

  # hide alert box
  dismiss: ->
    @unbind()
    @$el.finish().fadeOut 200, => @$el.css top: -1000

  # update position relative to thing this should be next to
  position: ->
    offset = @$nextTo.offset()

    if @cover
      @$el.css
        position: 'absolute'
        top:      offset.top + 'px'
        left:     offset.left + 'px'
        width:    @$nextTo.width()
        height:   @$nextTo.height()
    else
      topOffset = offset.top - $(window).scrollTop()
      @$el.css
        position: 'fixed'
        top:      (topOffset   - 42) + 'px'
        left:     (offset.left - 66) + 'px'

module.exports = AlertView
