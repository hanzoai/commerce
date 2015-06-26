crowdcontrol = require 'crowdcontrol'

moment = require 'moment'
util = require '../../util'
helpers = require '../helpers'

View = crowdcontrol.view.View

class BasicTableFieldView extends View
  tag: 'basic-table-field'
  html: require './basic-field.html'
  js: (opts)->
    @field = opts.field
    @value = opts.value
    @row = opts.row

new BasicTableFieldView

class NumericTableFieldView extends BasicTableFieldView
  tag: 'numeric-table-field'
  html: require './numeric-field.html'

new NumericTableFieldView

class MoneyTableFieldView extends NumericTableFieldView
  tag: 'money-table-field'
  js: ()->
    super
    @value = util.currency.renderUICurrencyFromJSON @row.currency, @value

new MoneyTableFieldView

class DateTableFieldView extends BasicTableFieldView
  tag: 'date-table-field'
  html: require './numeric-field.html'
  js: ()->
    super
    @value = moment(@value).format 'YYYY-MM-DD HH:mm'

new DateTableFieldView

# tag registration
helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'numeric'
, 'numeric-table-field'

helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'money'
, 'money-table-field'

helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'date'
, 'date-table-field'
