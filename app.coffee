express   = require 'express'
requisite = require 'requisite'

db       = require './db'
paypal   = require './paypal'
settings = require './settings'


app = express()


app.configure 'development', ->
  app.locals.pretty = true
  app.set 'views', __dirname
  app.set 'view engine', 'jade'
  app.use express.favicon()
  app.use express.logger 'dev'


app.configure ->
  app.use express.json()
  app.use express.urlencoded()
  app.use express.methodOverride()

  app.use '/api', (req, res, next) ->
    res.header 'Access-Control-Allow-Origin', '*'
    next()

  app.use express.static __dirname + '/storefront'
  app.use app.router
  app.use express.errorHandler()


app.get '/', (req, res) ->
  res.render 'storefront/donate/index'


app.get '/logged-in/:id', (req, res) ->
  if req.query.error?
    console.error 'login failed:', req.query.error_description
    return res.end req.query.error_description

  console.log 'logged-in'
  url = settings.extensions[req.params.id].redirect + '#' + req.query.code
  res.redirect url


app.get '/api/v1/get-user-info', (req, res) ->
  unless req.query.code?
    return res.json 'Missing authorization code', 401

  unless req.query.id?
    return res.json 'Missing id', 500

  {code, id} = req.query

  paypal.getAccessToken code, id, (err, token) ->
    console.log 'getAccessToken'
    return res.json err.toString(), 500 if err?

    paypal.getUserInfo token, (err, user) ->
      console.log 'getUserInfo'
      return res.json err.toString(), 500 if err?

      db.checkPurchase id, user.email, (err, purchased) ->
        return res.json err.toString(), 500 if err?

        res.json email: user.email, purchased: purchased


app.get '/api/v1/start-checkout', (req, res) ->
  unless req.query.amount?
    return res.json 'Missing amount', 500
  unless req.query.currency?
    return res.json 'Missing currency', 500
  unless req.query.id?
    return res.json 'Missing id', 500
  unless req.query.email?
    return res.json 'Missing email', 500

  console.log req.query

  {amount, currency, email, id} = req.query

  paypal.setExpressCheckout amount, currency, email, id, (err, token) ->
    return res.json err.toString(), 500 if err?

    res.json token: token


app.get '/api/v1/complete-checkout/:id/:details', (req, res) ->
  unless req.params.id?
    # not really appropriate but oh well
    return res.redirect 'http://paypal.com'

  {id} = req.params
  {cancelUrl, successUrl} = settings.extensions[id].paypal.expressCheckout

  unless req.query.token?
    return res.redirect cancelUrl

  unless req.params.details?
    return res.redirect cancelUrl

  try
    {PayerID, token} = req.query
    [amount, currency, email] = ((new Buffer(req.params.details, 'base64')).toString 'ascii').split ':'
  catch err
    res.redirect cancelUrl

  console.log req.query, amount, currency, email, id

  paypal.doExpressCheckoutPayment amount, currency, PayerID, token, (err) ->
    if err?
      console.error err
      res.redirect cancelUrl
    else
      res.redirect successUrl
      db.savePurchase email, id, (err) ->
        console.error err if err?

app.get '/api/v1/save-purchase', (req, res) ->
  {email, id} = req.query

  db.savePurchase email, id, (err) ->
    res.json err.toString(), 500 if err?


module.exports = app
