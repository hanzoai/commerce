_ = require 'underscore'
humanize = require 'humanize'

currencySigns = require './data/currencies'
currencySeparator = '.'
digitsOnlyRe = new RegExp('[^\\d.-]', 'g')

isZeroDecimal = (code)->
  if code == 'bif' || code == 'clp' || code == 'djf' || code == 'gnf' || code == 'jpy' || code == 'kmf' || code == 'krw' || code == 'mga' || code == 'pyg' || code == 'rwf' || code == 'vnd' || code == 'vuv' || code == 'xaf' || code == 'xof' || code == 'xpf'
    return true
  return false

module.exports = Util =
  getSymbol: (code)->
    return currencySigns[code] || ''

  renderUpdatedUICurrency: (code, uiCurrency)->
    return Util.renderUICurrencyFromJSON code, Util.renderJSONCurrencyFromUI(code, uiCurrency)

  renderUICurrencyFromJSON: (code, jsonCurrency)->
    if isNaN jsonCurrency
      jsonCurrency = 0
    currentCurrencySign = currencySigns[code] || ''

    jsonCurrency = '' + jsonCurrency
    # jsonCurrency is not cents
    if isZeroDecimal code
      return currentCurrencySign + jsonCurrency

    # jsonCurrency is cents
    while jsonCurrency.length < 3
      jsonCurrency = '0' + jsonCurrency

    return currentCurrencySign + humanize.numberFormat(jsonCurrency.substr(0, jsonCurrency.length - 2), 0) + '.' + jsonCurrency.substr(-2)

  renderJSONCurrencyFromUI: (code, uiCurrency) ->
    if _.isNumber(uiCurrency)
      return uiCurrency

    if isZeroDecimal code
      return parseInt(('' + uiCurrency).replace(digitsOnlyRe, '').replace(currencySeparator, ''), 10)

    # uiCurrency is a whole unit of currency
    parts = uiCurrency.split currencySeparator
    if parts.length > 1
      parts[1] = parts[1].substr(0, 2)
      while parts[1].length < 2
        parts[1] += '0'
    else
      parts[1] = '00'
    return parseInt(parseFloat(parts[0].replace(digitsOnlyRe, '')) * 100 + parseFloat(parts[1].replace(digitsOnlyRe, '')), 10)
