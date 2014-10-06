module.exports =
  hostname: 'checkout.verus.io'

  db:
    host:     'localhost'
    user:     'root'
    password: 'UsZXGHtvADP8V6nB'
    database: 'cloudstart'

  paypal:
    restApi:
      endpoint: 'api.sandbox.paypal.com'
    expressCheckout:
      endpoint: 'api-3t.sandbox.paypal.com'
      username:  'payments-facilitator_api1.404pagellc.com'
      password:  '1384567162'
      signature: 'AFcWxV21C7fd0v3bYYYRCpSSRl31AcK3kXHK-lAqVCN9AUbPC0lgERgn'

  extensions:
    0: # mailcheckerplus
      redirect: 'http://www.mailcheckerplus.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'AWy9URCykik2N8gWE-4yH6qYUa5CxjFSBzCVcR00d0krU3kby2LyQGkv4cqg'
          secret:   'EEYVuRBcdGzetzD1egiC4FSMnMHvvnBBwc54OeXJwWbWbCJTK6YYM3TN9xxJ'

        expressCheckout:
          desc:       'Mail Checker Plus User License'
          cancelUrl:  'http://www.mailcheckerplus.com/donate/#cancel'
          successUrl: 'http://www.mailcheckerplus.com/donate/#success'

    1: # scrolltotopbutton
      redirect: 'http://www.scrolltotopbutton.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'ARSjhBBckVO6BwHne204lBXquhOBxwpFs-WCIsEqTBw7TV8DGwcMI0IqAi2W'
          secret:   'EAVP5hBZf1N4moXYLnxw8dCF7WzJyKbGSb1te8Wz1Z6yjv0sLsvufTfkPf3N'

        expressCheckout:
          desc:       'Scroll To Top Button User License'
          cancelUrl:  'http://www.scrolltotopbutton.com/donate/#cancel'
          successUrl: 'http://www.scrolltotopbutton.com/donate/#success'

    2: # smoothgestures
      redirect: 'http://www.smoothgestures.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'ARoPGhAMN-4s8EluF1MgIq07w5mzGM0oI0zCO2jKSoviRT0CFqBgzI67hliF'
          secret:   'ECBHJBAk1_bgXvVQTPvdC-b_FwPIxaaiTRT4phD6CcheNvEJKD_fo-fo19wt'

        expressCheckout:
          desc:       'Smooth Gestures User License'
          cancelUrl:  'http://www.smoothgestures.com/donate/#cancel'
          successUrl: 'http://www.smoothgestures.com/donate/#success'

    3: # trollemoticons
      redirect: 'http://www.trollemoticons.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'AX4FgRBpLjQXxAkmEDLY9u5YGkx8RtfMdWPVfIAigpP0Xy9UEivd7IE93zi-'
          secret: 'EOpMcRD8hEPKbTF62Lj0YgZgbeiO8b2lCCkU7YNScIANLWssSUbtLplm8xEd'

        expressCheckout:
          desc:       'Troll Emoticons User License'
          cancelUrl:  'http://www.trollemoticons.com/donate/#cancel'
          successUrl: 'http://www.trollemoticons.com/donate/#success'

    4: # neatbookmarks
      redirect: 'http://www.neatbookmarksapp.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'AWPTkRAo_MwAZEaVPRvmXXizY9857NPcMLjD-szSDMbDVaTJex6FgA4-aw_I'
          secret:   'EKBTcxDYrs3ARJgVOUgOgihjAdot2Ruzl1ackxHGoqU5L--xEzD9h-DzgDVQ'

        expressCheckout:
          desc:       'Neat Bookmarks User License'
          cancelUrl:  'http://www.neatbookmarksapp.com/donate/#cancel'
          successUrl: 'http://www.neatbookmarksapp.com/donate/#success'

    6: # smoothscroll
      redirect: 'http://www.smoothscrollapp.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'Adp1iRBg_PQO9OSa_3sA3QmBpbAf7gE8Kt5ywTzeo3a1VDdwC95O0UT1xHPh'
          secret:   'EGohghD2zrq1VGXevHRKftejlir4EcGixmxKlwwVcI-qTxQ-UkOAledtgjdL'

        expressCheckout:
          desc:       'Smooth Scroll User License'
          cancelUrl:  'http://www.smoothscrollapp.com/donate/#cancel'
          successUrl: 'http://www.smoothscrollapp.com/donate/#success'
