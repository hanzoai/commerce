exports.humanizeNumber = (num) ->
  num.toString().replace /(\d)(?=(\d\d\d)+(?!\d))/g, "$1,"

exports.formatCurrency = (num) ->
  currency = num or 0
  humanizeNumber currency.toFixed(2)

_idCounter = 0
exports.uniqueId = (prefix) ->
  id = ++_idCounter + ''
  prefix ? prefix + id
