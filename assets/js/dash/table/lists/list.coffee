crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Api = crowdcontrol.data.Api
BasicTableView = table.BasicTableView
m = crowdcontrol.utils.mediator

class BasicList extends BasicTableView
  tag: 'basic-list'

  js: (opts)->
    super

    if opts.path?
      @path = opts.path if opts.path

      @api = api = Api.get 'crowdstart'

      m.trigger 'start-spin', @tag + @path + '-list-load'

      api.get(@path).then (res) =>
        m.trigger 'stop-spin', @tag + @path + '-list-load'
        @model = res.responseText
        @update()

module.exports = BasicList
