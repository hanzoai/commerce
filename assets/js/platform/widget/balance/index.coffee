_ = require 'underscore'
moment = require 'moment'
crowdcontrol = require 'crowdcontrol'

util = require '../../util'
table = require '../../table'
field = table.field

input = require '../../form/input'

Api = crowdcontrol.data.Api
View = crowdcontrol.view.View

BasicTableView = table.BasicTableView
FormView = crowdcontrol.view.form.FormView
m = crowdcontrol.utils.mediator

class BalanceWidgetFormView extends FormView
  tag: 'balance-widget-form'
  path: 'transaction'
  html: require './balance-form.html'
  inputConfigs: [
    input('type', '', 'required basic-select'),
    input('amount', 'ex 100', 'required money'),
    input('currency', '', 'currency-type-select'),
  ]
  js: (opts)->
    super
    @api = api = Api.get('crowdstart')

  _submit: ()->
    @api.post(@path, @model).then ()=>
      setTimeout ()=>
        @obs.trigger 'refresh'
      , 500

BalanceWidgetFormView.register()

class BalanceWidget extends View
  tag: 'balance-widget'
  html: require './template.html'

  currencyOptions: {}
  formModel:
    userId: ''
    type: 'deposit',
    amount: 0,
    currency: 'points'
  accountingOptions:
    deposit: 'Add(+)'
    withdraw: 'Subtract(-)'
  tableHeaders: [
    field('type', 'Type')
    field('amount', 'Amount', 'money')
    field('description', 'Description')
    field('createdAt', 'Created On', 'date')
  ]

  events:
    refresh: ()->
      m.trigger 'start-spin', 'balance-form-load'
      @api.get(@path).then (res) =>
        m.trigger 'stop-spin', 'balance-form-load'
        @updateModel res.responseText
      @update()

  updateModel: (model)->
    # We should only receive array models
    if !_.isArray(model) || model.length == 0
      return

    # prepare model
    model.sort (a, b)->
      return 1 if moment(a.createdAt).isBefore(b.createdAt)
      return -1

    # grab the last currency (most recently added)
    @currency = currency = model[0].currency

    newModel = {}
    for row in model
      transactions = newModel[row.currency]

      if !transactions
        transactions = newModel[row.currency] = []
      transactions.push row

      @currencyOptions[row.currency] = row.currency

    @model = newModel
    @obs.trigger BasicTableView.Events.NewData, newModel[currency]
    @update()

  change: (event)->
    @currency = $(event.target).val()
    @obs.trigger BasicTableView.Events.NewData, @model[@currency]
    @update()

  balance: ()->
    transactions = @model[@currency]

    amount = 0
    for transaction in transactions
      amount += if transaction.type == 'deposit' then transaction.amount else -transaction.amount

    return util.currency.renderUICurrencyFromJSON @currency, amount

  js: (opts)->
    #case sensitivity issues
    userId = opts.userId = opts.userId || opts.userid

    @path = "user/#{userId}/transactions"

    @api = Api.get('crowdstart')

    @obs.trigger 'refresh'

    @formModel.userId = userId

    @on 'update', ()=>
      $select = $($(@root).find('select')[0])
      if !@initialized && $select[0]?
        $select.chosen(
          width: '100%'
          disable_search_threshold: 3
        ).change((event)=>@change(event))
        @initialized = true
      requestAnimationFrame ()->
        $select.chosen().trigger("chosen:updated")

BalanceWidget.register()

# module.exports = BalanceWidget
