exports.humanizeNumber = (num) ->
  num.toString().replace /(\d)(?=(\d\d\d)+(?!\d))/g, "$1,"

exports.formatCurrency = (num) ->
  currency = num or 0
  humanizeNumber currency.toFixed(2)
