_ = require 'underscore'
humanize = require 'humanize'

BasicPanelView = require './basic'

class NumberPanel extends BasicPanelView
  tag: 'number-panel'
  decimals: 0
  js: (opts)->
    super

    @decimals = opts.decimals || @decimals || 0

  render: (val)->
    if !_.isNumber(val)
      return 0

    return humanize.numberFormat val, @decimals

NumberPanel.register()

module.exports = NumberPanel
