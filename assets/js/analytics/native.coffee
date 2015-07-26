do ->
  `var endpoint = '%s'`

  Espy = require 'espy'
  Cuckoo = require 'cuckoo'

  Cuckoo.Target 'click submit'
  Cuckoo.Egg (event)->
    type == event.type

    eventName = type + '_'

    if type == 'click' || type == 'submit'
      id = event.target.getAttribute 'id'
      if id != ''
        eventName += '#id'
      else
        name = event.target.getAttribute 'name'
        if name != ''
          eventName += "[name=#{name}]"
        else
          clas = event.target.getAttribute 'class'
          if clas != ''
            eventName = '.' + clas.replace(/ /g, '.')

      Espy.Event(eventName)

