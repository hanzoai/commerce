_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
BasicForm = require '../basic'
FormView = crowdcontrol.view.form.FormView

Api = crowdcontrol.data.Api

class Pane extends FormView
  tag: 'pane'
  path: ''

  html:     BasicForm.prototype.html
  events:   BasicForm.prototype.events
  reset: ()->
  js: (opts)->
    super

    @api = api = Api.get 'crowdstart'

    @on 'update', ()=>
      setTimeout ()=>
        $root = $(@root)
        $root.children().height $root.height()
      , 500

  queryString: ()->
    return ''

  _submit:  (event)->
    path = @path + '?q=' + @queryString()

    @api.get(path, @model).then((res)=>
      if res.status != 200
        throw new Error res.responseText.error.message

      data = res.responseText ? []
      @obs.trigger Events.Table.NewData, data
    ).catch (e)=>
      console.log(e.stack)
      @error = e
      # window.location.hash = @redirectPath


module.exports = Pane
