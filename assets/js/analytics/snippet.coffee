do ->
  return if window.analytics?

  analytics = []
  analytics.methods = [
    'ab'
    'alias'
    'group'
    'identify'
    'off'
    'on'
    'once'
    'page'
    'pageview'
    'ready'
    'track'
    'trackClick'
    'trackForm'
    'trackLink'
    'trackSubmit'
  ]

  for method in analytics.methods
    do (method) ->
      analytics[method] = ->
        args = Array::slice.call arguments
        args.unshift method
        analytics.push args
        analytics
      return

  script = document.createElement('script')
  script.async = true
  script.type = 'text/javascript'
  script.src = `%s`
  first = document.getElementsByTagName('script')[0]
  first.parentNode.insertBefore script, first

  window.analytics = analytics
