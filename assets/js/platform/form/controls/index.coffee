crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

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
  html: require '../../templates/backend/form/controls/static.html'

StaticView.register()

class StaticPreView extends StaticView
  tag: 'static-pre'
  html: require '../../templates/backend/form/controls/static-pre.html'

StaticPreView.register()

class StaticDateView extends StaticView
  tag: 'static-date'
  html: require '../../templates/backend/form/controls/static-date.html'

StaticDateView.register()

class IdLinkView extends StaticView
  tag: 'id-link'
  html: require '../../templates/backend/form/controls/id-link.html'
  js: (opts)->
    super
    @path = opts.input.model.cfg.hints['id-path']

IdLinkView.register()

class IdListLinkView extends IdLinkView
  tag: 'id-list-link'
  html: require '../../templates/backend/form/controls/id-list-link.html'

IdListLinkView.register()

class BasicInputView extends InputView
  errorHtml: ''
  tag: 'basic-input'
  html: require '../../templates/backend/form/controls/basic-input.html'
  js:(opts)->
    @model = if opts.input then opts.input.model else @model

BasicInputView.register()

class PasswordInputView extends BasicInputView
  tag: 'basic-password'
  html: require '../../templates/backend/form/controls/password.html'

PasswordInputView.register()

class NumericInputView extends BasicInputView
  tag: 'numeric-input'
  events:
    "#{Events.Input.Set}": (name, value) ->
      if name == @model.name
        @clearError()
        # in case the number was corrupted, reset to 0
        v = parseFloat(value)
        @model.value = if isNaN(v) then 0 else v
  js:(opts)->
    super

    v = parseFloat(@model.value)
    @model.value = if isNaN(v) then 0 else v

NumericInputView.register()

class DatePickerView extends BasicInputView
  tag: 'date-picker'

#localize here
# events:
  #   "#{Events.Input.Set}": (name, value) ->
  #     if name == @model.name

  js: (opts)->
    super

    @on 'update', ()=>
      $input = $(@root).find('input')
      if $input[0]?
        if !@initialized
          requestAnimationFrame ()=>
            $input.datepicker().on('changeDate', (event)=>@change(event))

          @initialized = true
      else
        requestAnimationFrame ()=>
          @update()

DatePickerView.register()

class BasicTextareaView extends BasicInputView
  tag: 'basic-textarea'
  html: require '../../templates/backend/form/controls/basic-textarea.html'

BasicTextareaView.register()

class CodeMirrorView extends BasicTextareaView
  tag: 'codemirror-js'
  js: (opts)->
    super

    @refresh()

    @on 'update', ()=>
      @refresh()

  refresh: ()->
    if @editor?
      @editor.refresh()
      return

    $el = $(@root).find('textarea')

    if $el[0]?
      @editor = CodeMirror.fromTextArea $el[0],
        lineNumbers: true,
        mode: "javascript",
        gutters: ["CodeMirror-lint-markers"],
        lint: true

      @editor.on 'change', (instance, changeObj)=>
        $el.val @editor.getValue()
        @change target: $el[0]


      requestAnimationFrame: ()=>
        @editor.refresh()

CodeMirrorView.register()

class Switch extends BasicInputView
  tag: 'switch'
  html: require '../../templates/backend/form/controls/switch.html'
  js: (opts)->
    @uid = '_' + Math.random()*10000

    super
  change: (event) ->
    value = event.target.checked
    if value == true || value == "true"
      value == true
    else
      value = false
    if value != @model.value
      @obs.trigger Events.Input.Change, @model.name, value
      @model.value = value
      @update()

Switch.register()

class DisabledInputView extends BasicInputView
  tag: 'disabled-input'
  html: require '../../templates/backend/form/controls/disabled-input.html'

DisabledInputView.register()

class MoneyInputView extends BasicInputView
  tag: 'money-input'

  events:
    "#{Events.Input.Set}": (name, value) ->
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
      @obs.trigger Events.Input.Change, @model.name, util.currency.renderJSONCurrencyFromUI(code, value)
      @model.value = value
      @update

  # get the currency set on the model (all models with currencies have both currency and amount field
  currency: (fn)->
    # convoluted return scheme
    @obs.trigger(Events.Input.Get, 'currency').one Events.Input.Result, (result)->
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
  html: require '../../templates/backend/form/controls/static.html'

StaticMoneyView.register()

class PercentInputView extends BasicInputView
  tag: 'percent-input'
  html: require '../../templates/backend/form/controls/percent-input.html'

  change: (event) ->
    value = @getValue(event.target)
    value = parseFloat(value)
    if isNaN value
      value = 0

    @obs.trigger Events.Input.Change, @model.name, value
    @model.value = value
    @update()

PercentInputView.register()

class BasicSelectView extends BasicInputView
  tag: 'basic-select'
  html: require '../../templates/backend/form/controls/basic-select.html'

  any: false
  tags: false

  # Use when loading options async
  async: false

  # These are used for caching values for async = true
  optionsLoaded: false
  lastValueSet: null

  events:
    "#{Events.Input.Set}": (name, value) ->
      if name == @model.name && value?
        @clearError()
        @model.value = value
        # whole page needs to be updated for side effects
        riot.update()
  options: ()->
    return @selectOptions

  changed: false
  change: (event) ->
    value = $(event.target).val()
    if value != @model.value
      @obs.trigger Events.Input.Change, @model.name, value
      @model.value = value
      @changed = true
      @update()

  isCustom: (o)->
    options = o
    if !options?
      options = @options()

    for name, value of options
      if _.isObject value
        if !@isCustom value
          return false

      else if name == @model.value
        return false

    return true

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

  initSelect: ($select)->
    $select.select2(
      tags: @tags
      placeholder: @model.placeholder
      minimumResultsForSearch: 10
    ).change((event)=>@change(event))

  js:(opts)->
    super

    @any = opts.any ? false
    @selectOptions = opts.options

    @on 'update', ()=>
      $select = $(@root).find('select')
      if $select[0]?
        if !@initialized
          requestAnimationFrame ()=>
            @initSelect($select)
            @initialized = true
            @changed = true
        else if @changed
          requestAnimationFrame ()=>
            if @async && !@optionsLoaded
              @lastValueSet = @model.value
            else
              # this bypasses caching of select option names
              # no other way to force select2 to flush cache
              if @isCustom()
                $select.select('destroy')
                @initSelect($select)
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
    countries = _.extend window.countries, {}
    countries['_any'] = 'Any Country' if @any
    return countries

CountrySelectView.register()

class CurrencySelectView extends BasicSelectView
  tag: 'currency-select'

  options: ()->
    currencies = _.extend window.currencies, {}
    currencies['_any'] = 'Any Currency' if @any
    return currencies

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
      @products = '_any': 'Any Product' if @any
      for product in res.responseText.models
        @products[product.id] = product.name

      @asyncDone()

    return @products = '_any': 'Any Product'

ProductSelectView.register()

class AnalyticsEventsSelect extends BasicSelectView
  tag: 'analytics-events-select'
  html: require '../../templates/backend/form/controls/optgroup-select.html'

  tags: true
  options: ()->
    return {
      'Standard Events':
        'page': 'Load/Page View'
        'Sign-up': 'Sign-up'
        'Logged In': 'Logged In'
        'Viewed Product': 'Viewed Product'
        'Added Product': 'Added Product'
        'Removed Product': 'Removed Product'
        'Completed Order': 'Completed Order'
        'Viewed Promotion': 'Viewed Promotion'
        'Clicked Promotion': 'Clicked Promotion'
      'E-Commerce Events':
        'Viewed Product Category': 'Viewed Product Category'
        'Viewed Checkout Step': 'Viewed Checkout Step'
        'Completed Checkout Step': 'Completed Checkout Step'
        'Viewed Checkout Step': 'Viewed Checkout Step'
        'Completed Checkout Step': 'Completed Checkout Step'
        'Viewed Checkout Step': 'Viewed Checkout Step'
        'Completed Checkout Step': 'Completed Checkout Step'
    }

AnalyticsEventsSelect.register()

class OrderStatusSelect extends BasicSelectView
  tag: 'order-status-select'
  options: ()->
    cancelled:  'Cancelled'
    completed:  'Completed'
    locked:     'Locked'
    'on-hold':  'On Hold'
    open:       'Open'

OrderStatusSelect.register()

class PaymentStatusSelect extends BasicSelectView
  tag: 'payment-status-select'
  options: ()->
    cancelled:  'Cancelled'
    credit:     'Credit'
    disputed:   'Disputed'
    failed:     'Failed'
    fraudulent: 'Fraudulent'
    paid:       'Paid'
    refunded:   'Refunded'
    unpaid:     'Unpaid'

PaymentStatusSelect.register()

class FulfillmentStatusSelect extends BasicSelectView
  tag: 'fulfillment-status-select'
  options: ()->
    unfulfilled:    'Unfulfilled'
    labelled:       'Labelled'
    processing:     'Processing'
    shipped:        'Shipped'
    delivered:      'Delivered'
    cancelled:      'Cancelled'

FulfillmentStatusSelect.register()

class ShippingServiceSelect extends BasicSelectView
  tag: 'shipping-service-select'
  options: ()->
    'GD':       'Domestic Ground'
    '2D':       'Domestic 2 Day'
    '1D':       'Domestic 1 Day'
    'E-INTL':   'International Economy'
    'INTL':     'International Standard'
    'PL-INTL':  'International Plus'
    'PM-INTL':  'International Premium'

ShippingServiceSelect.register()

# tag registration
helpers.registerTag (inputCfg)->
  return inputCfg.hints['switch']
, 'switch'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['text']
, 'basic-textarea'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['js']
, 'codemirror-js'

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
  return inputCfg.hints['date-picker']
, 'date-picker'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['mailinglist-thankyou-select']
, 'mailinglist-thankyou-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['currency-type-select']
, 'currency-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['analytics-events-select']
, 'analytics-events-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['order-status-select']
, 'order-status-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['payment-status-select']
, 'payment-status-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['fulfillment-status-select']
, 'fulfillment-status-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['shipping-service-select']
, 'shipping-service-select'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['static-money']
, 'static-money'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['static-date']
, 'static-date'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['static-pre']
, 'static-pre'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['static']
, 'static'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['id']
, 'id-link'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['id-list']
, 'id-list-link'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['numeric']
, 'numeric-input'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['money']
, 'money-input'

helpers.registerTag (inputCfg)->
  return inputCfg.hints['percent']
, 'percent-input'

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

helpers.registerValidator ((inputCfg) -> return inputCfg.hints['percent'])
, (model, name)->
  value = model[name]
  value = parseFloat(value)
  if isNaN value
    value = 0

  return value

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

helpers.registerValidator ((inputCfg) -> return inputCfg.hints['copy'])
, (model, name)->
  value = model[name]
  model[@hints.copy] =  value
  return value

helpers.registerValidator ((inputCfg) -> return inputCfg.hints['gtzero'])
, (model, name)->
  value = model[name]
  if value < 0
    return 0
  return value

# module.exports =
#   BasicInputView: BasicInputView
