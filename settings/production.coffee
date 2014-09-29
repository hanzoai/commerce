module.exports =
  hostname: 'k.yieldsquare.com'

  db:
    host:     'kaching.cr0jmxshfkhb.us-east-1.rds.amazonaws.com'
    user:     'kaching'
    password: 'TyR9Kac51kSd'
    database:  'kaching'

  paypal:
    restApi:
      endpoint: 'api.paypal.com'
    expressCheckout:
      endpoint: 'api-3t.paypal.com'
      username:  'jay_api1.404pagellc.com'
      password:  'KVPFB97KTPJLCKZR'
      signature: 'AcRjLHdPXRinWZ9gomkp88mjufwjAeoBSXY7Fvj5L8bfMqBgstwlZnaL'

  extensions:
    0: # mailcheckerplus
      redirect: 'http://www.mailcheckerplus.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'Ae3u1hCO9itUXNU7NJhJy5W-5vVIzF_Oyw12AbEbbvs_pZfkwpzWJ8O-GaCW'
          secret:   'EKISABAs3FVQ0OeM1NgyiL6OdeM-3OUkA13dS6D3q5itXQ8Eq_Ipt-WhNd24'

        expressCheckout:
          desc:       'Mail Checker Plus User License'
          cancelUrl:  'http://www.mailcheckerplus.com/donate/#cancel'
          successUrl: 'http://www.mailcheckerplus.com/donate/#success'

    1: # scrolltotopbutton
      redirect: 'http://www.scrolltotopbutton.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'Af37YxCbqdOZZOnBcjm2HUkAZHTN7e9NC_gujgXNfV8BdQ8gBciNU22LUmxA'
          secret: 'EKEdTRDKpyRXr9YvgVZz_x5B8l3wIZw_2twWQpXWwwoLDNoNlFupfvze1OGq'

        expressCheckout:
          desc:       'Scroll To Top Button User License'
          cancelUrl:  'http://www.scrolltotopbutton.com/donate/#cancel'
          successUrl: 'http://www.scrolltotopbutton.com/donate/#success'

    2: # smoothgestures
      redirect: 'http://www.smoothgestures.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'AYHjgRAmyNjcUAuiw2Unz-s5ordZl-6_Z5qF9eIOGLf2r6oCQEgFRfTlLEYJ'
          secret:   'ECSUnBCmVci1i3sVOPI1yAvpL-eAYsI6YZ6pW_nIYk5KP4MMxTdGwaxlfIzD'

        expressCheckout:
          desc:       'Smooth Gestures User License'
          cancelUrl:  'http://www.smoothgestures.com/donate/#cancel'
          successUrl: 'http://www.smoothgestures.com/donate/#success'

    3: # trollemoticons
      redirect: 'http://www.trollemoticons.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'AWEULBD_lqjd0G4kUV9a6Vba8v47DQrVgCoZFYbmqz0u-CBCORFPcXXHaGcV'
          secret:   'EKMeHxCae-FithHUm7rbb7pPKvtvfUioHiF7xBisz1pzCQ385UwywzLSh4VO'

        expressCheckout:
          desc:       'Troll Emoticons User License'
          cancelUrl:  'http://www.trollemoticons.com/donate/#cancel'
          successUrl: 'http://www.trollemoticons.com/donate/#success'

    4: # neatbookmarks
      redirect: 'http://www.neatbookmarksapp.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'ATTtDRAYbr9LPOkBLVmrZCTiR3dwJvIToSsDeT3fBkL5Um7TgvS2ybnzdtjo'
          secret:   'EBd6sRBwMZXM9tmb9VL60Jnj4XuCdbObaLDs9Q7Dnh9aQzRg_exKYUAb_MGX'

        expressCheckout:
          desc:       'Neat Bookmarks User License'
          cancelUrl:  'http://www.neatbookmarksapp.com/donate/#cancel'
          successUrl: 'http://www.neatbookmarksapp.com/donate/#success'

    6: # smoothscroll
      redirect: 'http://www.smoothscrollapp.com/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'AYWxAxCytMourNd7Yrs-QQkMuvouWf3fKSUCC9bmdJV5S3ZI_qUVycV5NhfS'
          secret:   'EOWUyhAGDClMBt91Tm29kO9o1bsl64JeSB6oz_xi6hFNk7gh__kiidRyLPfk'

        expressCheckout:
          desc:       'Smooth Scroll User License'
          cancelUrl:  'http://www.smoothscrollapp.com/donate/#cancel'
          successUrl: 'http://www.smoothscrollapp.com/donate/#success'
