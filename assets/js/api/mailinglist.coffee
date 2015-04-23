do ->
  # Embedded by MailingList Js() method
  endpoint = 'http://localhost:8080%s'
  thankyou = '%s'
  facebook =
    id:       '%s'
    value:    '%s'
    currency: '%s'
  google =
    category: '%s'
    name:     '%s'

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
      window._gaq.push ['_trackEvent', google.category, google.name]

    if window._fbq?
      window._fbq.push ['track', facebook.id,
        value:    facebook.value,
        currency: facebook.currency,
      ]


  addHandler = (e) ->
    console.log 'adding submit handler'

    form.removeEventListener 'submit', addHandler
    form.addEventListener 'submit', submitHandler

    setTimeout ->
      form.dispatchEvent new Event 'submit',
        bubbles:    false
        cancelable: false
    , 400

    e.preventDefault()
    return false

  redirect = ->
    setTimeout ->
      window.location = thankyou
    , 1000

  submitHandler = (e) ->
    console.log 'submit handler'

    return if e.defaultPrevented

    payload = JSON.stringify serialize form
    headers =
      'X-Requested-With': 'XMLHttpRequest',
      'Content-type':     'application/json; charset=utf-8'
      'Content-length':   payload.length
      'Connection':       'close'

    xhr = XHR()
    xhr.post endpoint, headers, payload, (err, status, xhr) ->
      return redirect() if status == 409
      return if err?

      # Fire tracking pixels
      track()
      redirect()

  # init
  form = getForm()
  form.addEventListener 'submit', addHandler
  return
