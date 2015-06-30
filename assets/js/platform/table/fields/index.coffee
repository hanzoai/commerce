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
    @value = opts.row[opts.field.id]
    @row = opts.row

BasicTableFieldView.register()

class IdTableFieldView extends BasicTableFieldView
  tag: 'id-table-field'
  html: require './link-field.html'
  js: (opts)->
    super
    @path = opts.field.hints['id-path']

IdTableFieldView.register()

class NumericTableFieldView extends BasicTableFieldView
  tag: 'numeric-table-field'
  html: require './numeric-field.html'

NumericTableFieldView.register()

class MoneyTableFieldView extends NumericTableFieldView
  tag: 'money-table-field'
  js: ()->
    super
    @value = util.currency.renderUICurrencyFromJSON @row.currency, @value

MoneyTableFieldView.register()

class DateTableFieldView extends BasicTableFieldView
  tag: 'date-table-field'
  html: require './numeric-field.html'
  js: ()->
    super
    @value = moment(@value).format 'YYYY-MM-DD HH:mm'

DateTableFieldView.register()

class AgoTableFieldView extends DateTableFieldView
  tag: 'ago-table-field'
  js: ()->
    super
    @value = $.timeago(@value)

AgoTableFieldView.register()

# tag registration
helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'numeric'
, 'numeric-table-field'

helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'money'
, 'money-table-field'

helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'ago'
, 'ago-table-field'

helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'date'
, 'date-table-field'

helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'id'
, 'id-table-field'
