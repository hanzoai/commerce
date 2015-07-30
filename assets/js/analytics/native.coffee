do ->
  Espy = require 'espy'
  Cuckoo = require 'cuckoo-js'

  Espy.url = '%%%%%url%%%%%'
  Cuckoo.Target 'click touch submit' # scroll'

  debounced = {}

  Cuckoo.Egg = (event)->
    type = event.type

    eventName = type

    if type == 'click' || type == 'touch' || type == 'submit'
      eventName += '_' + event.target.tagName
      id = event.target.getAttribute 'id'
      if id
        eventName += '#' + id
      else
        name = event.target.getAttribute 'name'
        if name
          eventName += "[name=#{name}]"
        else
          clas = event.target.getAttribute 'class'
          if clas
            eventName += '.' + clas.replace(/ /g, '.')

      if !debounced[eventName]?
        Espy eventName
    else if type == 'scroll'
      if !debounced[eventName]?
        Espy eventName,
          scrollX: window.scrollX
          scrollY: window.scrollY

    debounced[eventName] = setTimeout ()->
      debounced[eventName] = undefined
    , 100
