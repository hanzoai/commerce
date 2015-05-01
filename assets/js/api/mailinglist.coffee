do ->
  `var endpoint = "%s", ml = %s` # Embedded by MailingList Js() method

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

  getForm = ->
    # start at the root element
    node = document.documentElement

    # find last HTMLElement child node
    node = node.lastChild  while node.childNodes.length and node.lastChild.nodeType is 1

    # node is now the script element
    form = node.parentNode
    window.form = form

  serialize = (form) ->
    return {} if not form or form.nodeName isnt 'FORM'

    data =
      metadata: {}

    elements = form.getElementsByTagName 'input'

    # loop over form elements
    for el in elements
      k = el.name.trim().toLowerCase()
      v = el.value.trim()
      unless k and v
        continue

      if /email/.test v
        data.email = v
      else
        data.metadata[k] = v

    unless data.email?
      throw new Error 'No email provided, make sure form element has an email field and that the value is populated correctly'

    data

  fb = (opts) ->
    unless window._fbq?
      window._fbq = []
      fbds = document.createElement 'script'
      fbds.async = true
      fbds.src = '//connect.facebook.net/en_US/fbds.js'
      s = document.getElementsByTagName('script')[0]
      s.parentNode.insertBefore fbds, s
      _fbq.loaded = true

    window._fbq.push ['track', opts.id,
      value:    opts.value,
      currency: opts.currency,
    ]

  ga = (opts)->
    unless window._gaq?
      window._gaq = []
      ga = document.createElement 'script'
      ga.type = 'text/javascript'
      ga.async = true
      ga.src = ((if 'https:' is document.location.protocol then 'https://' else 'http://')) + 'stats.g.doubleclick.net/dc.js'
      s = document.getElementsByTagName('script')[0]
      s.parentNode.insertBefore ga, s

    window._gaq.push ['_trackEvent', opts.category, opts.name]

  track = ->
    ga ml.google if ml.google.category?
    fb ml.facebook if ml.facebook.id?

  addHandler = (ev) ->
    form.removeEventListener 'submit', addHandler
    form.addEventListener    'submit', submitHandler

    setTimeout ->
      form.dispatchEvent new Event 'submit',
        bubbles:    false
        cancelable: true
    , 500

    ev.preventDefault()
    false

  thankYou = ->
    switch ml.thankyou.type
      when 'redirect'
        setTimeout ->
          window.location = ml.thankyou.url
        , 1000
      when 'html'
        form.innerHTML = ml.thankyou.html

  submitHandler = (ev) ->
    if ev.defaultPrevented
      return
    else
      ev.preventDefault()

    payload = JSON.stringify serialize form
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

  # init
  form = getForm()
  form.addEventListener 'submit', addHandler
  return
