types = require './types'
helpers = require './helpers'

require './fields'

module.exports =
  helpers: helpers

  BasicTableView:   types.BasicTableView
  TableFieldConfig: types.TableFieldConfig

  field: (id, name, type, hints)->
    return new types.TableFieldConfig(id, name, type, hints)
