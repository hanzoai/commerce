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

  # get form container
  getContainer = (script, selector = '')->
    if selector != ''
      document.querySelector selectors.container
    else
      parent = script.parentNode
      inputs = parent.getElementsByTagName 'input'
      if inputs.length < 1
        document.body
      else
        parent

  # get this script tag
  getScript = ->
    # find last script node
    scripts = document.getElementsByTagName( 'script' )

    # last element is this script tag
    script = scripts[ scripts.length - 1 ]
    script

  # Get elements from inside a parent
  getElements = (parent, selector) ->
    if selector? and selector != ''
      parent.querySelectorAll selector
    else
      [parent]

  # get value from a selector
  getValue = (parent = document.body, selector) ->
    el = parent.querySelector selector
    el?.value?.trim()

  # serialize a form
  serializeForm = (form) ->
    return {} unless form?

    data =
      metadata: {}

    # Loop over form elements
    for el in form.elements
      try
        # Clean up inputs
        k = el.name.trim().toLowerCase()
        v = el.value.trim()
        type = el.getAttribute('type').toLowerCase()

        if (type == 'checkbox' || type == 'radio') && !el.checked
          return

        # Skip inputs we don't care about
        if k == '' or v == '' or type == 'submit'
          continue

        # Detect emails
        if /email/.test k
          data.email = v
        else
          data.metadata[k] = v
      catch e
        console.log "Skipping valueless form input"

    # Use selectors if necessary
    if selectors.email
      data.email = getValue form, selectors.email
    else
      data.email ?= ''

    for prop in ['firstname', 'lastname', 'name']
      if (selector = selectors[prop])?
        data.metadata[prop] = getValue form, selector

    data

  # get setting off script tag data attribute
  attr = (name) ->
    val = script.getAttribute 'data-' + name
    return unless val?

    switch val.trim().toLowerCase()
      when 'false'
        false
      when 'true'
        true
      else
        val

  # Trigger event tracking
  track = ->
    return unless typeof analytics?.track is 'function'
    analytics.track 'Lead', category: 'Subscription'

  # Wire up submit handler
  addHandler = (el, errorEl) ->
    unless errorEl?
      errorEl               = document.createElement 'div'
      errorEl.className     = 'crowdstart-mailinglist-error'
      errorEl.style.display = 'none'
      errorEl.style.width   = 100 + '%%'[0]  # Prevents interpolation from picking up % as a thing needing interpolatin'
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

      if document.createEvent && document.dispatchEvent
        try
          el.dispatchEvent new Event 'thankyou',
            bubbles:    true
            cancelable: true
        catch e
          event = document.createEvent 'Event'
          event.initEvent 'thankyou', true, true
          document.dispatchEvent event
      else
        console.log "Could not create or dispatch thankyou event"

    submitHandler = (ev) ->
      if ev.defaultPrevented
        return
      else
        ev.preventDefault()

      data = serializeForm el

      if ml.validate
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
        return showError(err) if err?

        # Fire tracking pixels
        track()
        thankYou()

      false

    (ev) ->
      el.removeEventListener 'submit', addHandler
      el.addEventListener    'submit', submitHandler

      setTimeout ->
        if document.createEvent && document.dispatchEvent
          try
            el.dispatchEvent new Event 'submit',
              bubbles:    false
              cancelable: true
          catch e
            # try it the terrible IE way
            event = document.createEvent 'Event'
            event.initEvent 'submit', false, true
            el.dispatchEvent event
        else
          console.log "Could not create or dispatch submit event"
      , 500

      ev.preventDefault()
      false

  # Init all the things
  init = ->
    if called then return else called = true

    props = ['container', 'forms', 'submits', 'errors', 'email', 'firstname', 'lastname', 'name']
    for prop in props
      selectors[prop] = (attr prop) ? ml.selectors?[prop]

    # default selector for submit button
    selectors.submits ?= 'input[type="submit"], button[type="submit"]'

    # are we validating?
    ml.validate ?= (attr 'validate') ? false

    parent   = getContainer script, selectors.container
    forms    = getElements parent, selectors.forms
    handlers = getElements parent, selectors.submits

    # find error divs
    if selectors.errors
      errors = getElements parent, selectors.errors
    else
      errors = []

    for handler, i in handlers
      do (handler, i) ->
        return if handler.getAttribute 'data-hijacked'

        handler.setAttribute 'data-hijacked', true
        handler.addEventListener 'click',  (addHandler forms[i], errors[i])
        handler.addEventListener 'submit', (addHandler forms[i], errors[i])

  # Get script tag, has to run before rest of DOM loads
  script = getScript()

  # Run init after DOM loads, attach various listeners
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
