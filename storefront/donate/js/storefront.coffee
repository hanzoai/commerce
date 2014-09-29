updateDonateButton = (title, message) ->
  ($ '#donate-button-lg .donate-here').text title
  ($ '#donate-button-lg .turn-off-ads').text message

class Application
  constructor: (@opts) ->

  start: ->
    @router = new Router()
    Backbone.history.start()

  navigate: (url) ->
    # required to make backbone actually trigger next url...bug?
    @router.navigate 'navigating...', trigger: true
    @router.navigate url, trigger: true

  loggedIn: (id, code) ->
    console.log 'logged in', id, code
    window.user = @user = new User id: id, code: code
    @swapView new LoadingView()
    @user.fetch
      success: =>
        if @user.get 'purchased'
          @swapView new DonatedView()
        else
          @swapView new CheckoutView()
      error: ->
        console.log 'ERROR: Failed to fetch user data'

  swapView: (view) ->
    (($ '#donate-action').html '').append view.el

  startCheckout: (amount, currency) ->
    url = "http://#{@opts.kachingUrl}/api/v1/start-checkout?amount=#{amount}&currency=#{currency}&email=#{@user.get 'email'}&id=#{@opts.extension.id}"
    $.getJSON url, (data) =>
      console.log data
      {token} = data

      window.location.href = "https://#{@opts.paypal.expressCheckoutEndpoint}/cgi-bin/webscr?cmd=_express-checkout&token=#{token}"

  fireDonated: (data) ->
    id = setInterval ->
      el = document.getElementById 'donated-event'
      if el?
        clearInterval id

        console.log 'firing donated event', data
        ev = document.createEvent 'Event'
        ev.initEvent 'donated', true, true
        el.innerText = data
        el.dispatchEvent ev
    , 100


class User extends Backbone.Model
  url: ->
    "#{@urlRoot()}?code=#{@get 'code'}&id=#{@get 'id'}"

  urlRoot: ->
    "http://#{app.opts.kachingUrl}/api/v1/get-user-info"


class StaticView extends Backbone.View
  initialize: ->
    @render()

  render: ->
    @$el.html @template opts: app.opts
    @


class ChoiceView extends StaticView
  id: 'choice-view'
  template: require './templates/choice'

  initialize: ->
    super
    paypal.use ['login'], (login) ->
      login.render app.opts.paypal.loginButton

  events:
    'click #choice-ad':         'choiceAd'
    'click #login-with-paypal': 'choiceDonate'

  choiceAd: (e) ->
    $('#donate-plea').html ''
    app.swapView new AdView()

  choiceDonate: (e) ->
    app.swapView new LoginView()


class UpgradeView extends StaticView
  className: 'upgrade-view'
  template: require './templates/upgrade'

  render: ->
    super
    ($ '#donate-headline').html ''


class AdView extends StaticView
  className: 'span10'
  template: require './templates/ad-accept'

  render: ->
    super
    updateDonateButton 'THANK YOU', 'Ad support is now on'


class LoginView extends StaticView
  className: 'span10'
  template: require './templates/login'
  events:
    'click #retry': (ev) ->
      app.navigate '/'
      ev.preventDefault()


class CheckoutView extends StaticView
  className: 'span10'
  template: require './templates/checkout'
  events:
    'change input#slider':    'onSliderInput'
    'click #checkout-button': 'onCheckoutButtonClick'

  # set initial amount from value of slider input
  render: ->
    super
    @slider = @$el.find 'input#slider'
    @sliderAmount = @$el.find '#slider-amount'
    @onSliderInput()

  onSliderInput: ->
    @sliderAmount.text(@formatCurrency(@slider.val()))

  onCheckoutButtonClick: ->
    amount = @slider.val()
    currency = 'USD'
    app.startCheckout amount, currency

  formatCurrency: (value) ->
    "$" + parseFloat(value).toFixed(2).toString()


class DonatedView extends StaticView
  className: 'span10 donated-view'
  template: require './templates/donated'
  render: ->
    super
    updateDonateButton 'THANK YOU', 'Ad support is now off'
    ($ '#donate-plea').html ''
    app.fireDonated 'donated'


class LoadingView extends StaticView
  className: 'span10'
  template: require './templates/loading'


class SuccessView extends StaticView
  className: 'span10 success-view'
  template: require './templates/success'
  render: ->
    super
    updateDonateButton 'THANK YOU', 'Ad support is now off'
    ($ '#donate-plea').html ''
    app.fireDonated 'success'


class CancelView extends StaticView
  className: 'span10 cancel-view'
  template: require './templates/cancel'
  events:
    'click #retry':         'onRetry'
  onRetry: (ev) ->
    app.navigate '/'
    ev.preventDefault()


class ErrorView extends StaticView
  className: 'span10'
  template: require './templates/error'

  events:
    'click #retry':         'onRetry'

  onRetry: (ev) ->
    app.navigate '/'
    ev.preventDefault()


class Router extends Backbone.Router
  routes:
    '':          'choice'
    'cancel?*q': 'cancel'
    'checkout':  'checkout'
    'error':     'error'
    'success':   'success'
    'upgrade':   'upgrade'

  choice: ->
    app.swapView new ChoiceView()

  checkout: ->
    app.swapView new CheckoutView()

  success: ->
    app.swapView new SuccessView()

  cancel: ->
    app.swapView new CancelView()

  error: ->
    app.swapView new ErrorView()

  upgrade: ->
    app.swapView new ChoiceView()
    view = new UpgradeView el: $('#one')

#
# deprecate in favor of storefront.start(opts)
#
window.storefrontStart = (opts) ->
  window.app = new Application opts
  app.start()

window.storefront =
  start: (opts) ->
    window.app = new Application opts
    app.start()
  UpgradeView: UpgradeView
