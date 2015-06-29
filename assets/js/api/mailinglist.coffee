do ->
  `var endpoint = "%s", ml = %s` # Embedded by MailingList Js() method

  called    = false
  errors    = null
  forms     = null
  handlers  = null
  parent    = null
  script    = null
  selectors = {}
  validate  = false

  XHR = ->
    xhr = null

    if window.XMLHttpRequest
      xhr = new XMLHttpRequest()

    else if window.ActiveXObject
      xhr = new ActiveXObject 'Microsoft.XMLHTTP'

    setHeaders: (headers) ->
      for k,v of headers
        xhr.setRequestHeader k, v
      return

    post: (url, headers, payload, cb) ->
      xhr.open 'POST', url, true
      @setHeaders headers
      xhr.send payload

      xhr.onreadystatechange = ->
        if xhr.readyState == 4
          if xhr.status == 200 or xhr.status == 201
            cb null, xhr.status, xhr
          else
            cb (new Error 'Subscription failed'), xhr.status, xhr
        return
      return

  getScript = ->
    # start at the root element
    node = document.documentElement

    # find last HTMLElement child node
    node = node.lastChild while node.childNodes.length and node.lastChild.nodeType is 1

    # last HTMLElement is script tag
    node

  getElements = (parent, selector) ->
    console.log 'getElements', parent, selector

    if selector? and selector != ''
      # look up form elements
      console.log parent.querySelectorAll selector
      parent.querySelectorAll selector
    else
      console.log [parent]
      [parent]

  getValue = (selector, el = document) ->
    console.log 'getValue', selector, el
    found = el.querySelector selector
    console.log found, found?.value?.trim()
    found?.value?.trim()

  serialize = (el) ->
    return {} unless el?

    data =
      metadata: {}

    inputs = el.getElementsByTagName 'input'

    # Loop over form elements
    for input in inputs
      # Clean up inputs
      k = input.name.trim().toLowerCase()
      v = input.value.trim()

      # Skip inputs we don't care about
      if k == '' or v == '' or (input.getAttribute 'type') == 'submit'
        continue

      # Detect emails
      if /email/.test k
        data.email = v
      else
        data.metadata[k] = v

    # Use selectors if necessary
    if selectors.email
      data.email = getValue selectors.email, el

    for prop in ['firstname', 'lastname', 'name']
      if (selector = selectors[prop])?
        data.metadata[prop] = getValue selector, el

    console.error 'Email is required' unless data.email?

    data

  attr = (s) ->
    script.getAttribute 'data-' + s

  google =
    setup: ->
      return if window.ga? or window._gaq?

      ((i, s, o, g, r, a, m) ->
        i['GoogleAnalyticsObject'] = r
        i[r] = i[r] or ->
          (i[r].q = i[r].q or []).push arguments
          return

        i[r].l = 1 * new Date()

        a = s.createElement(o)
        m = s.getElementsByTagName(o)[0]

        a.async = 1
        a.src = g
        m.parentNode.insertBefore a, m
        return
      ) window, document, 'script', '//www.google-analytics.com/analytics.js', 'ga'
      return

    track: (opts) ->
      return unless opts.category?

      google.setup()

      category = opts.category ? 'Subscription'
      action   = opts.action   ? opts.name ? 'Signup'
      label    = opts.label    ? ''

      if window._gaq?
        window._gaq.push ['_trackEvent', category, action]
      if window.ga?
        window.ga 'send', 'event', category, action, label, 0

  facebook =
    setup: ->
      return if window._fbq?.loaded

      _fbq = window._fbq or (window._fbq = [])

      fbds = document.createElement('script')
      fbds.async = true
      fbds.src = '//connect.facebook.net/en_US/fbds.js'
      s = document.getElementsByTagName('script')[0]
      s.parentNode.insertBefore fbds, s
      _fbq.loaded = true
      return

    track: (opts) ->
      return unless opts.id?

      facebook.setup()

      window._fbq.push ['track', opts.id,
        value:    opts.value,
        currency: opts.currency,
      ]
      return

  track = ->
    facebook.track ml.facebook
    google.track ml.google
    return

  addHandler = (el, errorEl) ->
    unless errorEl?
      errorEl = document.createElement 'div'
      errorEl.className = 'crowdstart-mailinglist-error'
      errorEl.style.display = 'none'
      errorEl.style.width   = '100%'
      el.appendChild errorEl

    showError = (msg) ->
      errorEl.style.display   = 'inline'
      errorEl.innerHTML = msg
      false

    hideError = ->
      errorEl.style.display = 'none'

    thankYou = ->
      switch ml.thankyou.type
        when 'redirect'
          setTimeout ->
            window.location = ml.thankyou.url
          , 1000
        when 'html'
          el.innerHTML = ml.thankyou.html

    submitHandler = (ev) ->
      if ev.defaultPrevented
        return
      else
        ev.preventDefault()

      data = serialize el
      console.log data

      if validate
        unless data.email?
          return showError 'Email is required'
        if (data.email.indexOf '@') == -1
          return showError 'Invalid email'
        if data.email.length < 3
          return showError 'Invalid email'
        hideError()

      payload = JSON.stringify data

      headers =
        'X-Requested-With': 'XMLHttpRequest',
        'Content-type':     'application/json; charset=utf-8'

      xhr = XHR()
      xhr.post endpoint, headers, payload, (err, status, xhr) ->
        return thankYou() if status == 409
        return if err?

        # Fire tracking pixels
        track()
        thankYou()

      false

    (ev) ->
      el.removeEventListener 'submit', addHandler
      el.addEventListener    'submit', submitHandler

      setTimeout ->
        el.dispatchEvent new Event 'submit',
          bubbles:    false
          cancelable: true
      , 500

      ev.preventDefault()
      false

  init = ->
    if called then return else called = true

    window.parent = parent = script.parentNode
    console.log 'parent', parent, parent.querySelectorAll '.form-group'

    props = ['forms', 'submits', 'errors', 'email', 'firstname', 'lastname', 'name']
    for prop in props
      selectors[prop] = (attr prop) ? ml.selectors?[prop]

    console.log 'selectors', selectors

    # are we validating?
    validate = (attr 'validate') ? ml.validate ? ''

    # data attributes can only be strings
    validate = false if validate?.toLowerCase() == 'false'

    # init
    forms    = getElements parent, selectors.forms
    handlers = getElements parent, selectors.submits

    # error handling
    if selectors.errors
      errors = getElements parent, selectors.errors
    else
      errors = []

    console.log 'forms', forms
    console.log 'handlers', handlers
    console.log 'errors', errors

    for handler, i in handlers
      do (handler, i) ->
        return if handler.getAttribute 'data-hijacked'

        handler.setAttribute 'data-hijacked', true
        handler.addEventListener 'click',  (addHandler forms[i], errors[i])
        handler.addEventListener 'submit', (addHandler forms[i], errors[i])

    console.log selectors

  # get script tag
  script = getScript()

  # setup listeners for load event
  if document.addEventListener
    document.addEventListener 'DOMContentLoaded', init, false
  else if document.attachEvent
    document.attachEvent 'onreadystatechange', ->
      init() if document.readyState == 'complete'

  if window.addEventListener
      window.addEventListener 'load', init, false
  else if window.attachEvent
      window.attachEvent 'onload', init

  return
