riot = require 'riot'
_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

tokenize = crowdcontrol.view.form.tokenize

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

  constructor: (@id, @name, @type='text', hints = '')->
    @hints = tokenize hints

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

Events.Table =
  PrepareForNewData: 'table-prepare'
  NewData: 'table-new-data'
  StartSearch: 'table-start-search'
  EndSearch: 'table-end-search'

class BasicTableView extends View
  tag: 'basic-table'
  html: require '../templates/backend/table/template.html'
  searching: false
  events:
    "#{Events.Table.NewData}": (model)->
      @model = model
      riot.update()
    "#{Events.Table.StartSearch}": ()->
      @searching = true
    "#{Events.Table.EndSearch}": ()->
      @searching = false
  isEmpty: ()->
    model = @model
    return !model? || !model.length || model.length == 0
  js: (opts)->
    @headers ?= opts.headers
    @headerMap = {}
    for header in @headers
      @headerMap[header.id] = header

BasicTableView.register()

module.exports =
  BasicTableView:   BasicTableView
  TableFieldConfig: TableFieldConfig
  field: (id, name, type, hints)->
    return new TableFieldConfig(id, name, type, hints)

