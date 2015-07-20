do ->
  `var config = %s` # Embedded by Organization.AnalyticsJs() method

  # Google Analytics
  do (i = window, s = document, o = 'script', g = '//www.google-analytics.com/analytics.js', r = 'ga', a, m) ->
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

  ga 'create', config.google.trackingId, 'auto'
  ga 'send', 'pageview'

  # Facebook Remarketing
  do (f = window, b = document, e = 'script', v = '//connect.facebook.net/en_US/fbevents.js', n, t, s) ->
    return if f.fbq

    n = f.fbq = ->
      (if n.callMethod then n.callMethod.apply(n, arguments) else n.queue.push(arguments))
      return

    f._fbq = n unless f._fbq
    n.push = n
    n.loaded = not 0
    n.version = '2.0'
    n.queue = []
    t = b.createElement(e)
    t.async = not 0
    t.src = v
    s = b.getElementsByTagName(e)[0]
    s.parentNode.insertBefore t, s
    return

  fbq 'init', cfg.facebook.remarketingId
  fbq 'track', 'PageView'
