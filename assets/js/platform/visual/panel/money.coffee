_ = require 'underscore'
humanize = require 'humanize'

BasicPanelView = require './basic'
util = require '../../util'

class MoneyPanel extends BasicPanelView
  tag: 'money-panel'
  html: require '../../templates/backend/visual/panel/money.html'

  decimals: 0

  loadData: (model, compareModel, currency)->
    @currency = currency
    super model, compareModel

  js: (opts)->
    super

    @decimals = opts.decimals || @decimals || 0

  render: (val)->
    return util.currency.renderUpdatedUICurrency @currency, val

MoneyPanel.register()

module.exports = MoneyPanel
