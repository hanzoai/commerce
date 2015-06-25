types = require './types'
helpers = require './helpers'

require './fields'

module.exports =
  helpers: helpers

  TableView:        types.TableView
  TableFieldConfig: types.TableFieldConfig

  field: (id, name, type, hints)->
    return new types.TableFieldConfig(id, name, type, hints)
