crowdcontrol = require 'crowdcontrol'
_ = require 'underscore'
riot = require 'riot'

util = require '../../util'

Api = crowdcontrol.data.Api
View = crowdcontrol.view.View
InputView = crowdcontrol.view.form.InputView

helpers = crowdcontrol.view.form.helpers
helpers.defaultTagName = 'basic-input'

# views
class StaticView extends InputView
  tag: 'static'
  html: require './static.html'

StaticView.register()

class StaticDateView extends StaticView
  tag: 'static-date'
  html: require './static-date.html'

StaticDateView.register()

class IdLinkView extends StaticView
  tag: 'id-link'
  html: require './id-link.html'
  js: (opts)->
    super
    @path = opts.input.model.cfg.hints['id-path']

IdLinkView.register()

class BasicInputView extends InputView
  errorHtml: ''
  tag: 'basic-input'
  html: require './basic-input.html'
  js:(opts)->
    @model = if opts.input then opts.input.model else @model

BasicInputView.register()

class BasicTextareaView extends BasicInputView
  tag: 'basic-textarea'
  html: require './basic-textarea.html'

BasicTextareaView.register()

class Switch extends BasicInputView
  tag: 'switch'
  html: require './switch.html'
  change: (event) ->
    value = event.target.checked
    if value != @model.value
      @obs.trigger InputView.Events.Change, @model.name, value
      @model.value = value
      @update()

Switch.register()

class DisabledInputView extends BasicInputView
  tag: 'disabled-input'
  html: require './disabled-input.html'

DisabledInputView.register()

class MoneyInputView extends BasicInputView
  tag: 'money-input'

  events:
    "#{InputView.Events.Set}": (name, value) ->
      if name == @model.name
        @clearError()
        # in case the number was corrupted, reset to 0
        value = if isNaN(parseFloat(value)) then 0 else value
        @currency (code)=>
          @model.value = util.currency.renderUICurrencyFromJSON(code, value)
          @update()

  change: (event) ->
    value = @getValue(event.target)
    @currency (code)=>
      @obs.trigger InputView.Events.Change, @model.name, util.currency.renderJSONCurrencyFromUI(code, value)
      @model.value = value
      @update

  # get the currency set on the model (all models with currencies have both currency and amount field
  currency: (fn)->
    # convoluted return scheme
    @obs.trigger(InputView.Events.Get, 'currency').one InputView.Events.Result, (result)->
      fn(result)

  js:(opts)->
    @model = if opts.input then opts.input.model else @model
    model = @model
    @currency (code)->
      model.value = util.currency.renderUICurrencyFromJSON(code, model.value)

    @on 'update', ()=>
      @currency (code)=>
        value = util.currency.renderUpdatedUICurrency(code, model.value)
        if value != model.value
          model.value = value
          @update()

MoneyInputView.register()

class StaticMoneyView extends MoneyInputView
  tag: 'static-money'
  html: require './static.html'

StaticMoneyView.register()

class BasicSelectView extends BasicInputView
  tag: 'basic-select'
  html: require './basic-select.html'
  options: ()->
    @selectOptions
  changed: false
  change: (event) ->
    value = $(event.target).val()
    if value != @model.value
      @obs.trigger InputView.Events.Change, @model.name, value
      @model.value = value
      @changed = true
      @update()

  js:(opts)->
    super

    @selectOptions = opts.options

    @on 'update', ()=>
      $select = $(@root).find('select')
      if $select[0]?
        if !@initialized
          $select.select2(
            placeholder: @model.placeholder
            minimumResultsForSearch: 10
          ).change((event)=>@change(event))
          @initialized = true
          @changed = true
        else if @changed
          requestAnimationFrame ()=>
            $select.select2('val', @model.value)
            @changed = false
      else
        requestAnimationFrame ()=>
          @update()

    @on 'unmount', ()=>
      $select = $(@root).find('select')

BasicSelectView.register()

class CountrySelectView extends BasicSelectView
  tag: 'country-select'
  events:
    "#{InputView.Events.Set}": (name, value) ->
      if name == @model.name
        @clearError()
        @model.value = value
        # whole page needs to be updated for side effects
        riot.update()
  options: ()->
    return window.countries

CountrySelectView.register()

class CurrencySelectView extends BasicSelectView
  tag: 'currency-select'
  events:
    "#{InputView.Events.Set}": (name, value) ->
      if name == @model.name
        @clearError()
        @model.value = value
        # whole page needs to be updated for side effects
        riot.update()
  options: ()->
    return window.currencies

CurrencySelectView.register()

# tag registration
helpers.registerTag (inputCfg)->
  return inputCfg.hints['switch']
, 'switch'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['text']
, 'basic-textarea'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['disabled']
, 'disabled-input'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['basic-select']
, 'basic-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['country-select']
, 'country-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['currency-type-select']
, 'currency-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['static-money']
, 'static-money'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['static-date']
, 'static-date'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['static']
, 'static'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['id']
, 'id-link'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['money']
, 'money-input'

# validator registration
helpers.registerValidator ((inputCfg) -> return inputCfg.hints['numeric'])
, (model, name)->
  value = model[name]
  return parseFloat(value)

helpers.registerValidator ((inputCfg) -> return inputCfg.hints['required'])
, (model, name)->
  value = model[name]
  if _.isNumber(value)
    return value

  value = value?.trim()
  throw new Error "Required" if !value? || value == ''

  return value

helpers.registerValidator ((inputCfg) -> return inputCfg.hints['email'])
, (model, name)->
  value = model[name]
  value = value?.trim().toLowerCase()
  re = /[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*@(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?/
  if value? && value.match(re)
    return value
  throw new Error "Enter a valid email"

helpers.registerValidator ((inputCfg) -> return inputCfg.hints['money'])
, (model, name)->
  value = model[name]
  return parseFloat(value)

# should be okay for single one of these on a form
helpers.registerValidator ((inputCfg) ->return inputCfg.hints['email-unique'])
, (model, name)->
  value = model[name]
  if value == @hints['email-unique-exception']
    return value

  return Api.get('crowdstart').get('account/exists/' + value).then (res)->
    if res.responseText.exists
      throw new Error "Email already exists"
    return value
  , ()->
    return value
  return value

helpers.registerValidator ((inputCfg) ->return inputCfg.hints['unique'])
, (model, name)->
  value = model[name]
  if value == @hints['unique-exception']
    return value

  return Api.get('crowdstart').get(@hints['unique-api'] + '/' + value).then (res)->
    if res.status == 200 || res.staticText == 'OK'
      throw new Error "'#{value}' is already in use"
    return value
  , ()->
    return value
  return value

# module.exports =
#   BasicInputView: BasicInputView
