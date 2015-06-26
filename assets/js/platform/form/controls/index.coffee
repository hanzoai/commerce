crowdcontrol = require 'crowdcontrol'
_ = require 'underscore'
riot = require 'riot'

util = require '../../util'

View = crowdcontrol.view.View
InputView = crowdcontrol.view.form.InputView

helpers = crowdcontrol.view.form.helpers
helpers.defaultTagName = 'basic-input'

# views
class StaticView extends InputView
  tag: 'static'
  html: require './static.html'

new StaticView

class StaticDateView extends StaticView
  tag: 'static-date'
  html: require './static-date.html'

new StaticDateView

class BasicInputView extends InputView
  errorHtml: ''
  tag: 'basic-input'
  html: require './basic-input.html'
  js:(opts)->
    @model = if opts.input then opts.input.model else @model

new BasicInputView

class MoneyInputView extends BasicInputView
  tag: 'money-input'

  events:
    "#{InputView.Events.Set}": (name, value) ->
      if name == @model.name
        @clearError()
        # in case the number was corrupted, reset to 0
        value = if isNaN(parseFloat(value)) then 0 else value
        code = @view.currency()
        @model.value = util.currency.renderUICurrencyFromJSON(code, value)
        @update()

  mixins:
    change: (event) ->
      value = @view.getValue(event.target)
      code = @view.currency()
      @obs.trigger InputView.Events.Change, @model.name, util.currency.renderJSONCurrencyFromUI(code, value)
      @model.value = value

  # get the currency set on the model (all models with currencies have both currency and amount field
  currency: ()->
    # convoluted return scheme
    @curr = {value: ''}
    @ctx.obs.trigger(InputView.Events.Get, 'currency').one InputView.Events.Result, (result)=>
      @curr.value = result
    return @curr.value

  js:(opts)->
    @model = if opts.input then opts.input.model else @model
    model = @model
    code = @view.currency()
    model.value = util.currency.renderUICurrencyFromJSON(code, model.value)

    @on 'update', ()->
      code = @view.currency()
      model.value = util.currency.renderUpdatedUICurrency(code, model.value)

new MoneyInputView

class BasicSelectView extends BasicInputView
  tag: 'basic-select'
  html: require './basic-select.html'
  mixins:
    options: ()->
      @selectOptions
  js:(opts)->
    super

    @selectOptions = opts.options

    @on 'update', ()=>
      $select = $(@root).find('select')
      if !@initialized && $select[0]?
        $select.chosen(
          width: '100%'
          disable_search_threshold: 3
        ).change((event)=>@change(event))
        @initialized = true
      requestAnimationFrame ()->
        $select.chosen().trigger("chosen:updated")

new BasicSelectView

class CountrySelectView extends BasicSelectView
  tag: 'country-select'
  events:
    "#{InputView.Events.Set}": (name, value) ->
      if name == @model.name
        @clearError()
        @model.value = value
        # whole page needs to be updated for side effects
        riot.update()
  mixins:
    options: ()->
      return window.countries

new CountrySelectView

class CurrencySelectView extends BasicSelectView
  tag: 'currency-select'
  events:
    "#{InputView.Events.Set}": (name, value) ->
      if name == @model.name
        @clearError()
        @model.value = value
        # whole page needs to be updated for side effects
        riot.update()
  mixins:
    options: ()->
      return window.currencies

new CurrencySelectView

tokenize = (str)->
  tokens = str.split(' ')
  dict = {}
  for token in tokens
    if token.indexOf(':') >= 0
      [k, v] = token.split(':')
      dict[k] = v
    else
      dict[token] = true

  return dict

# tag registration
helpers.registerTag (inputCfg)->
  return inputCfg.hints.indexOf('basic-select') >= 0
, 'basic-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints.indexOf('country-select') >= 0
, 'country-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints.indexOf('currency-type-select') >= 0
, 'currency-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints.indexOf('static-date') >= 0
, 'static-date'

helpers.registerTag (inputCfg)->
  return inputCfg.hints.indexOf('static') >= 0
, 'static'


helpers.registerTag (inputCfg)->
  return inputCfg.hints.indexOf('money') >= 0
, 'money-input'

# validator registration
helpers.registerValidator ((inputCfg) -> return inputCfg.hints.indexOf('required') >= 0), (model, name)->
  value = model[name]
  if _.isNumber(value)
    return value

  value = value.trim()
  throw new Error "Required" if !value? || value == ''

  return value

helpers.registerValidator ((inputCfg) -> return inputCfg.hints.indexOf('email') >= 0), (model, name)->
  value = model[name]
  value = value.trim().toLowerCase()
  re = /[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*@(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?/
  if value.match(re)
    return value
  throw new Error "Enter a valid email"

helpers.registerValidator ((inputCfg) -> return inputCfg.hints.indexOf('money') >= 0), (model, name)->
  value = model[name]
  return parseFloat(value)

# should be okay for single one of these on a form
emailExceptConfig = null
helpers.registerValidator (inputCfg) ->
  hints = tokenize(inputCfg.hints)
  if hints['email-unique']
    emailExceptConfig = inputCfg
    return true
  return false
, (model, name)->
  value = model[name]
  if emailExceptConfig?
    hints = tokenize(emailExceptConfig.hints)

    if value == hints['email-unique-exception']
      return value

    return crowdcontrol.config.api.get('account/exists/' + value).then (data)->
      if data.data.exists
        throw new Error "Email already exists"
      return value
    , ()->
      return value
  return value

# module.exports =
#   BasicInputView: BasicInputView
