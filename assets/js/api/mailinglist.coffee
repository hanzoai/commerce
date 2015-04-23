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
    console.log 'serializing form'

    # start at the root element
    node = document.documentElement

    # find last HTMLElement child node
    node = node.lastChild  while node.childNodes.length and node.lastChild.nodeType is 1

    # node is now the script element
    form = node.parentNode
    window.form = form

  serialize = (form) ->
    console.log 'serializing form'

    return {} if not form or form.nodeName isnt "FORM"
    data = {}

    elements = form.getElementsByTagName 'input'
    console.log elements

    for el in elements
      data[el.name] = el.value.trim()

    data

  track = ->
    console.log 'tracking event'

    if window._gaq?
      window._gaq.push ['_trackEvent', ml.google.category, ml.google.name]

    if window._fbq?
      window._fbq.push ['track', ml.facebook.id,
        value:    ml.facebook.value,
        currency: ml.facebook.currency,
      ]


  addHandler = (ev) ->
    console.log 'adding submit handler'

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
    console.log 'submit handler'

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
