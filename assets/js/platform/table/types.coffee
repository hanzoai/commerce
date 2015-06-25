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

class TableView extends View
  tag: 'basic-table'
  html: require './template.html'

  js: (opts)->
    @headers = opts.headers

    if _.isArray(@model)
      @loading = true
    else
      @loading = false
      src = opts.src
      if !src?
        src = new Source
          api: crowdcontrol.config.api || opts.api
          path: opts.path
          policy: opts.policy || crowdcontrol.data.Policy.Once

      src.on Source.Events.Loading, ()=>
        @loading = true
        @update()

      src.on Source.Events.LoadData, (data)=>
        @loading = false
        if !_.isArray(data)
          throw new Error 'TableView needs an array of models'
        @model = data
        @update()

new TableView

module.exports =
  TableView:        TableView
  TableFieldConfig: TableFieldConfig

