_ = require 'underscore'

BasicPanelView = require './basic'
util = require '../../util'

class MoneyPanel extends BasicPanelView
  tag: 'money-panel'
  html: require '../../templates/dash/visual/panel/money.html'

  decimals: 0

  loadData: (model, compareModel, currency)->
    @currency = currency
    super model, compareModel

  js: (opts)->
    super

    @decimals = opts.decimals || @decimals || 0

  render: (val)->
    v = (util.currency.renderUpdatedUICurrency @currency, val)
    v = v.substring(0, v.indexOf('.'))

MoneyPanel.register()

module.exports = MoneyPanel
