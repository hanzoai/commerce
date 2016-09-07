crowdcontrol = require 'crowdcontrol'

moment = require 'moment'
util = require '../../util'
helpers = require '../helpers'

View = crowdcontrol.view.View
Api = crowdcontrol.data.Api

class BasicTableFieldView extends View
  tag: 'basic-table-field'
  html: require '../../templates/backend/table/fields/basic-field.html'
  js: (opts)->
    @field = opts.field
    @row = row = opts.row

    id = opts.field.id
    names = id.split '.'

    if names.length == 1
      @value = row[id]
    else
      currentObject = row
      for name in names
        if !currentObject[name]?
          @value = undefined
          return

        currentObject = currentObject[name]

      @value = currentObject

BasicTableFieldView.register()

class TextareaTableFieldView extends BasicTableFieldView
  tag: 'textarea-table-field'
  html: require '../../templates/backend/table/fields/textarea-field.html'

TextareaTableFieldView.register()

class SnippetTableFieldView extends TextareaTableFieldView
  tag: 'snippet-table-field'
  js: (opts)->
    super

    api = Api.get('crowdstart')

    @value = '<script src="' + api.url + '/mailinglist/' + @value + '/js"></script>'

SnippetTableFieldView.register()

class IdTableFieldView extends BasicTableFieldView
  tag: 'id-table-field'
  html: require '../../templates/backend/table/fields/link-field.html'
  js: (opts)->
    super
    @displayField = opts.field.hints['id-display']
    @displayValue =  if @displayField? then opts.row[@displayField] else opts.row[@field.id]
    @path = opts.field.hints['id-path']

IdTableFieldView.register()

class IdListTableFieldView extends BasicTableFieldView
  tag: 'id-list-table-field'
  html: require '../../templates/backend/table/fields/link-list-field.html'
  js: (opts)->
    super
    @path = opts.field.hints['id-path']

IdListTableFieldView.register()

class NumericTableFieldView extends BasicTableFieldView
  tag: 'numeric-table-field'
  html: require '../../templates/backend/table/fields/numeric-field.html'

NumericTableFieldView.register()

class MoneyTableFieldView extends NumericTableFieldView
  tag: 'money-table-field'
  js: ()->
    super
    @value = util.currency.renderUICurrencyFromJSON @row.currency, @value

MoneyTableFieldView.register()

class TotalTableFieldView extends NumericTableFieldView
  tag: 'total-table-field'
  js: ()->
    super
    @value = util.currency.renderUICurrencyFromJSON @row.currency, @row.price * @row.quantity

TotalTableFieldView.register()

class DateTableFieldView extends BasicTableFieldView
  tag: 'date-table-field'
  html: require '../../templates/backend/table/fields/numeric-field.html'
  js: ()->
    super
    @value = moment(@value).format 'YYYY-MM-DD HH:mm'

DateTableFieldView.register()

class AgoTableFieldView extends DateTableFieldView
  tag: 'ago-table-field'
  js: ()->
    super
    @value = moment(@value).fromNow()

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
  return fieldCfg.type == 'total'
, 'total-table-field'

helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'id'
, 'id-table-field'

helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'id-list'
, 'id-list-table-field'

helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'textarea'
, 'textarea-table-field'

helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'snippet'
, 'snippet-table-field'
