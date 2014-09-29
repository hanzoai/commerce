querystring = require 'querystring'
request     = require 'crequest'
{extensions, hostname, paypal} = require './settings'


module.exports =
  # get access token for user from authorization code
  getAccessToken: (code, id, cb) ->
    url = "https://#{paypal.restApi.endpoint}/v1/identity/openidconnect/tokenservice/"
    form =
      grant_type:   'authorization_code'
      code:         code
      redirect_uri: "http://#{hostname}/logged-in/#{id}"

    opts =
      auth:
        user: extensions[id].paypal.restApi.clientId
        pass: extensions[id].paypal.restApi.secret
      form: form

    request.post url, opts, (err, res, body) ->
      return cb err if err?

      if res.statusCode != 200
        err = new Error 'Request failed with ' + res.statusCode
        err.body = body
        return cb err

      cb null, body.access_token

  # get user info with access token
  getUserInfo: (token, cb) ->
    url = "https://#{paypal.restApi.endpoint}/v1/identity/openidconnect/userinfo/?schema=openid"
    opts =
      headers:
        Authorization: 'Bearer ' + token

    request url, opts, (err, res, body) ->
      return cb err if err?

      if res.statusCode != 200
        err = new Error 'Request failed with ' + res.statusCode
        err.body = body
        return cb err

      cb null, body

  setExpressCheckout: (amount, currency, email, id, cb) ->
    url = "https://#{paypal.expressCheckout.endpoint}/nvp"

    details = (new Buffer("#{amount}:#{currency}:#{email}")).toString('base64').replace /\=/g, ''

    opts =
      form:
        'USER':                           paypal.expressCheckout.username
        'PWD':                            paypal.expressCheckout.password
        'SIGNATURE':                      paypal.expressCheckout.signature
        'METHOD':                         'SetExpressCheckout'
        'VERSION':                        93
        'PAYMENTREQUEST_0_PAYMENTACTION': 'SALE'
        'PAYMENTREQUEST_0_AMT':           amount
        'PAYMENTREQUEST_0_ITEMAMT':       amount
        'PAYMENTREQUEST_0_CURRENCYCODE':  currency
        'PAYMENTREQUEST_0_DESC':          extensions[id].paypal.expressCheckout.desc
        'L_PAYMENTREQUEST_0_NAME0':       extensions[id].paypal.expressCheckout.desc
        'L_PAYMENTREQUEST_0_QTY0':        1
        'L_PAYMENTREQUEST_0_AMT0':        amount
        'NOSHIPPING':                     1
        'ALLOWNOTE':                      0
        'RETURNURL':                      "http://#{hostname}/api/v1/complete-checkout/#{id}/#{details}"
        'CANCELURL':                      extensions[id].paypal.expressCheckout.cancelUrl
        'SOLUTIONTYPE':                   'Sole'

    request.post url, opts, (err, res, body) ->
      return cb err if err?

      if res.statusCode != 200
        err = new Error 'Request failed with ' + res.statusCode
        err.body = body
        return cb err

      parsed = querystring.parse body
      if parsed.ACK == 'Success'
        cb null, parsed.TOKEN
      else
        err = new Error 'Request failed with ' + parsed.L_LONGMESSAGE0
        err.body = parsed
        cb err

  getExpressCheckoutDetails: (token, cb) ->
    console.log 'getExpressCheckoutDetails'
    url = "https://#{paypal.expressCheckout.endpoint}/nvp"

    opts =
      form:
        'USER':      paypal.expressCheckout.username
        'PWD':       paypal.expressCheckout.password
        'SIGNATURE': paypal.expressCheckout.signature
        'METHOD':    'GetExpressCheckoutDetails'
        'VERSION':   93
        'TOKEN':     token

    request.post url, opts, (err, res, body) ->
      return cb err if err?

      if res.statusCode != 200
        err = new Error 'Request failed with ' + res.statusCode
        err.body = body
        return cb err

      cb null, body

  doExpressCheckoutPayment: (amount, currency, payerId, token, cb) ->
    console.log 'doExpressCheckoutPayment'
    url = "https://#{paypal.expressCheckout.endpoint}/nvp"

    opts =
      form:
        'USER':                           paypal.expressCheckout.username
        'PWD':                            paypal.expressCheckout.password
        'SIGNATURE':                      paypal.expressCheckout.signature
        'METHOD':                         'DoExpressCheckoutPayment'
        'VERSION':                        93
        'TOKEN':                          token
        'PAYERID':                        payerId
        'PAYMENTREQUEST_0_PAYMENTACTION': 'Sale'
        'PAYMENTREQUEST_0_AMT':           amount
        'PAYMENTREQUEST_0_CURRENCYCODE':  currency

    request.post url, opts, (err, res, body) ->
      return cb err if err?

      parsed = querystring.parse body

      if parsed.ACK == 'Success'
        cb null
      else
        err = new Error 'Request failed with ' + parsed.L_LONGMESSAGE0
        err.body = parsed
        cb err
