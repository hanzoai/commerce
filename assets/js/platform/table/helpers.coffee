riot = require 'riot'
_ = require 'underscore'

class TableFieldCondition
  constructor: (@predicate, @tagName)->

helpers =
  # tagLookup contains a list of predicate tagName pairs
  tagLookup: []

  # defaultTagName specifies what tag name is set if no lookup predicate is satisfied
  defaultTagName: 'basic-table-field'

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

      if lookup.predicate tableFieldConfig
        tag = lookup.tagName
        return tag

    return @defaultTagName

riot.tag 'table-field', '', (opts)->
  field = opts.field

  tag = helpers.render field

  dummyOpts = _.extend {}, opts

  tags = riot.mount @root, tag, opts

  #hack :\
  tags[0].js opts

module.exports = helpers
