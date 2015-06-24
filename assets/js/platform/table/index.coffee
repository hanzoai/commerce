_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

class TableFieldCondition
  constructor: (@predicate, @tagName)->

helpers =
  # tagLookup contains a list of predicate tagName pairs
  tagLookup: []

  # defaultTagName specifies what tag name is set if no lookup predicate is satisfied
  defaultTagName: 'cs-table-text-field'

  # registerTag takes a predicate of type (InputConfig) -> bool and tagName
  registerTag: (predicate, tagName)->
    @tagLookup.push new TableFieldCondition(predicate, tagName)

  # delete an existing lookup
  deleteTag: (tagName)->
    for lookup, i in @tagLookup
      if lookup.tagName == tagName
        @tagLookup[i] = null

  # render a TableFieldConfig object, a tag
  render: (tableFieldConfig)->
    if !tableFieldConfig?
      return

    for lookup in @tagLookup
      if !lookup?
        continue

      if lookup.predicate TableFieldConfig
        tag = lookup.tagName
        return tag

    return @defaultTagName

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

class TableFieldView extends crowdcontrol.view.View

riot.tag "table-field", "", (opts)->
  field = opts.field
  value = opts.value

  tag = helpers.render field

  riot.mount @root, tag, opts

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

class TableView
  tag: 'cs-table'
  html: require './template.html'

  js: (opts)->
    @headers = opts.headers

    if _.isArray(@model)
      @loading = true
    else
      @loading = false
      src = new Source
        api: crowdstart.config.api
        path: opts.path
        policy: opts.policy || crowdstart.data.Policy.Once

      src.on Source.Events.Loading, ()=>
        @loading = true
        @update()

      src.on Source.Events.LoadData, (data)=>
        @loading = false
        if !_.isArray(data)
          throw new Error 'TableView needs an array of models'
        @model = data
        @update()

module.exports =
  helpers: helpers

  TableView:        TableView
  TableFieldView:   TableFieldView
  TableFieldConfig: TableFieldConfig
