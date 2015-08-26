_ = require 'underscore'
humanize = require 'humanize'

BasicPanelView = require './basic'
util = require '../../util'

class MoneyPanel extends BasicPanelView
  tag: 'money-panel'
  decimals: 0

  loadData: (model, compareModel)->
    @currency = ''

    for currency, cents of model
      if currency != ''
        @currency = currency
        break

    if @currency == ''
      super 0, 0
      return

    super model[@currency], compareModel[@currency]

  js: (opts)->
    super

    @decimals = opts.decimals || @decimals || 0

  render: (val)->
    return util.currency.renderUpdatedUICurrency @currency, val

MoneyPanel.register()

module.exports = MoneyPanel
