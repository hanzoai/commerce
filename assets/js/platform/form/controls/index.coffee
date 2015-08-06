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

class PasswordInputView extends BasicInputView
  tag: 'basic-password'
  html: require './password.html'

PasswordInputView.register()

class NumericInputView extends BasicInputView
  tag: 'numeric-input'
  events:
    "#{InputView.Events.Set}": (name, value) ->
      if name == @model.name
        @clearError()
        # in case the number was corrupted, reset to 0
        v = parseFloat(value)
        @model.value = if isNaN(v) then 0 else v
  js:(opts)->
    @model = if opts.input then opts.input.model else @model
    v = parseFloat(@model.value)
    @model.value = if isNaN(v) then 0 else v

NumericInputView.register()

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

  # Use when loading options async
  async: false

  # These are used for caching values for async = true
  optionsLoaded: false
  lastValueSet: null

  events:
    "#{InputView.Events.Set}": (name, value) ->
      if name == @model.name && value?
        @clearError()
        @model.value = value
        # whole page needs to be updated for side effects
        riot.update()
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

  # if async is set to true, then call this when async loading is done
  asyncDone: ()->
    @optionsLoaded = true
    # Only use lastValueSet if it is set to something
    #
    # Notes:
    # 1) @model.value is usually set by the form directly in initFormGroup
    #
    # 2) In the case of a form that is POSTing to create a new item,
    #   @model.value is set directly in initFormGroup during riot.mount
    #   before async loading of the select values compltes.
    #   Nothing else needs to happen here and @lastValueSet will not be set
    #
    # 3) In the case of a form that is PUTing to update an existing model,
    #   @model.value is set directly BUT a series of updates fires when the models
    #   are loaded.  This causes async select2('val', ...) calls before the async
    #   loading is done for the select options.  This is the case where we need to cache
    #   the value from the model and send it to select2('val', ...) by setting @model.value
    #   and firing update.
    #
    if @lastValueSet?
      @model.value = @lastValueSet
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
            if @async && !@optionsLoaded
              @lastValueSet = @model.value
            else
              $select.select2('val', @model.value)
              @changed = false
      else
        requestAnimationFrame ()=>
          @update()

    @on 'unmount', ()=>
      $select = $(@root).find('select')

BasicSelectView.register()

class MailinglistThankyouSelectView extends BasicSelectView
  tag: 'mailinglist-thankyou-select'
  options: ()->
    return {
      html: 'Show HTML Template where form was.'
      redirect: 'Redirect to URL'
      disable: 'Use the default form action.'
    }

MailinglistThankyouSelectView.register()

class CountrySelectView extends BasicSelectView
  tag: 'country-select'
  options: ()->
    return window.countries

CountrySelectView.register()

class CurrencySelectView extends BasicSelectView
  tag: 'currency-select'
  options: ()->
    return window.currencies

CurrencySelectView.register()

class CouponTypeSelectView extends BasicSelectView
  tag: 'coupon-type-select'
  options: ()->
    return {
      flat: 'Flat (Deduct Amount from Product\'s Price in Product\'s Currency)'
      percent: 'Percent (Deduct Percent from Product\'s Price)'
      'free-shipping': 'Free Shipping'
    }

CouponTypeSelectView.register()

class ProductSelectView extends BasicSelectView
  tag: 'product-type-select'
  async: true
  options: ()->
    return @products if @products?

    api = Api.get('crowdstart')
    api.get('product').then (res)=>
      @products = '_': 'All'
      for product in res.responseText.models
        @products[product.id] = product.name

      @asyncDone()

    return @products = '_': 'All'

ProductSelectView.register()

# tag registration
helpers.registerTag (inputCfg)->
  return inputCfg.hints['switch']
, 'switch'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['text']
, 'basic-textarea'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['password']
, 'basic-password'

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
  return inputCfg.hints['coupon-type-select']
, 'coupon-type-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['product-type-select']
, 'product-type-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['mailinglist-thankyou-select']
, 'mailinglist-thankyou-select'

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
  return inputCfg.hints['numeric']
, 'numeric-input'

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

helpers.registerValidator ((inputCfg) -> return inputCfg.hints['min'])
, (model, name)->
  value = model[name]
  if value? & value.length >= parseInt @hints['min'], 10
    return value
  throw new Error "Minimum Length is " + @hints['min']

helpers.registerValidator ((inputCfg) -> return inputCfg.hints['password-match'])
, (model, name)->
  value = model[name]
  value2 = model[@hints['password-match']]
  if value == value2
    return value
  throw new Error "Your passwords must match"

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

helpers.registerValidator ((inputCfg) -> return inputCfg.hints['copy'])
, (model, name)->
  value = model[name]
  model[@hints.copy] =  value
  return value

# module.exports =
#   BasicInputView: BasicInputView
