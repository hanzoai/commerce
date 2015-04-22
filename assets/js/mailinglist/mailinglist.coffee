do ->
  url = '%s/%s/subscribe'

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
    form

  serialize = (form) ->
    return {} if not form or form.nodeName isnt "FORM"
    data = {}

    for el in form.elements
      data[el.name] = el.value.trim()

    data

  bind = ->
    form = getForm()

    form.onsubmit = ->
      payload = JSON.stringify serialize form
      headers =
        'X-Requested-With': 'XMLHttpRequest',
        'Content-type':     'application/json; charset=utf-8'
        'Content-length':   payload.length
        'Connection':       'close'

      xhr = XHR()
      xhr.post url, headers, payload, (err, status, xhr) ->
        console.log xhr

    return

  bind()
