riot = require 'riot'
_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

View = crowdcontrol.view.View
Source = crowdcontrol.data.Source

class TableFieldConfig
  # field in the model
  field: ''
  # name display name of the field
  name: ''
  # type of field
  type: ''
  # hints for the tagLookup
  hints: ''

  constructor: (@id, @name, @type='text', @hints = '')->

# Model needs to be in the form of:
#
# TableView requires the following fields
#   header: a list of TableFieldConfigs in the model to display - [{name:'', field: '', type: ''}, ...]
#   model: array of models to display in the table.  Use only if there is static data
#     [
#       {
#         something: ''
#         somethingelse: ''
#       }
#     ]
#   path: path relative to api.  Use only if retrieving live data

TableViewEvents =
  NewData: 'table-new-data'

class BasicTableView extends View
  @Events: TableViewEvents

  tag: 'basic-table'
  html: require './template.html'
  events:
    "#{TableViewEvents.NewData}": (model)->
      @model = model
      @update()
  mixin:
    isEmpty: ()->
      model = @model
      return model? && model.length && model.length > 0
  js: (opts)->
    @headers = opts.headers

new BasicTableView

module.exports =
  BasicTableView:   BasicTableView
  TableFieldConfig: TableFieldConfig

