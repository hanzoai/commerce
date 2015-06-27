crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicTableView = table.BasicTableView
m = crowdcontrol.utils.mediator

class BasicList extends BasicTableView
  tag: 'basic-list'

  js: (opts)->
    @path = opts.path if opts.path

    if opts.src?
      @src = src = src
    else if @isEmpty()
      @src = src = new Source
        name: @tag + @path + 'order-list'
        api: crowdcontrol.config.api || opts.api
        path: @path
        policy: opts.policy || crowdcontrol.data.Policy.Once

    if src?
      src.on Source.Events.Loading, ()=>
        m.trigger 'start-spin', @tag + @path + '-list-load'
        @update()

      src.on Source.Events.LoadData, (model)=>
        m.trigger 'stop-spin', @tag + @path + '-list-load'
        @model = model
        @update()

module.exports = BasicList
