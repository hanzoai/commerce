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
      endpoint:  'api-3t.sandbox.paypal.com'
      username:  'paypal-facilitator_api1.verus.io'
      password:  '1412022809'
      signature: 'An5ns1Kso7MWUdW4ErQKJJJ4qi4-AfYMWWkK4Zy4f8IxXgjdthkvmMSC'

  extensions:
    0: # mailcheckerplus
      redirect: 'http://checkout.verus.io/donate/logged-in.html'

      paypal:
        restApi:
          clientId: 'AamaZBD0vqMZBS8NVn8Wa-EVhNo5Lx6eOLc3WrnGotHCeRdqxjZASg8I8jNY'
          secret:   'ELI9uxAuUAYrYvj8h_Nhhcpdxsie8okC18KsCL4JnHzpZ2GsL6qpqlxRXeo2'

        expressCheckout:
          desc:       'SKULLY AR-1'
          cancelUrl:  'http://checkout.verus.io/#cancel'
          successUrl: 'http://checkout.verus.io/#success'
