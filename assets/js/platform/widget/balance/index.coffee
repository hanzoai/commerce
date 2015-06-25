table = require '../../table'
field = table.field

crowdcontrol = require 'crowdcontrol'

input = require '../../form/input'

View = crowdcontrol.view.View
Source = crowdcontrol.data.Source

FormView = crowdcontrol.view.form.FormView

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
    @api = crowdcontrol.config.api || opts.api

  submit: ()->
    @ctx.api.post(@path, @ctx.model)

new BalanceWidgetFormView

class BalanceWidget extends View
  tag: 'balance-widget'
  html: require './template.html'

  js: (opts)->
    #case sensitivity issues
    userId = opts.userId = opts.userId || opts.userid

    path = "user/#{userId}/transactions"

    @loading = false
    @src = src = new Source
      api: crowdcontrol.config.api || opts.api
      path: path
      policy: opts.policy || crowdcontrol.data.Policy.Once

    src.on Source.Events.Loading, ()=>
      @loading = true
      @update()

    src.on Source.Events.LoadData, (data)=>
      @loading = false
      @model = data
      @update()

    @formModel =
      userId: userId
      type: 'deposit',
      amount: 0,
      currency: 'points'

    @accountingOptions =
      deposit: 'Add(+)'
      withdraw: 'Subtract(-)'

    @tableHeaders = [
      field('type', 'Type')
      field('amount', 'Amount', 'numeric')
    ]

new BalanceWidget

module.exports = BalanceWidget
