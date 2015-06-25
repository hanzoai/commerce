crowdcontrol = require 'crowdcontrol'

helpers = require '../helpers'

View = crowdcontrol.view.View

class BasicTableFieldView extends View
  tag: 'basic-table-field'
  html: require './basic-field.html'
  js: (opts)->
    @field = opts.field
    @value = opts.value

new BasicTableFieldView

class NumericTableFieldView extends BasicTableFieldView
  tag: 'numeric-table-field'
  html: require './numeric-field.html'

new NumericTableFieldView

# tag registration
helpers.registerTag (fieldCfg)->
  return fieldCfg.type == 'numeric' || fieldCfg.type == 'money'
, 'numeric-table-field'


