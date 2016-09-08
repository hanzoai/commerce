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

    @tableObs = opts.tableobs ? @obs

    @api = api = Api.get 'crowdstart'

    @on 'update', ()=>
      setTimeout ()=>
        $root = $(@root)
        $root.children().attr('style', '')
        requestAnimationFrame ()->
          $root.children().height $root.height()
      , 500

  queryString: ()->
    return ''

  _submit:  (event)->
    @searching = true
    @update()

    path = @path + '?q=' + @queryString()
    path += "&limit=1000" if window.User.owner

    @tableObs.trigger Events.Table.PrepareForNewData

    @api.get(path, @model).then((res)=>
      @searching = false
      @update()

      if res.status != 200
        throw new Error res.responseText.error.message

      data = res.responseText ? []
      @tableObs.trigger Events.Table.NewData, data
    ).catch (e)=>
      console.log(e.stack)
      @error = e
      # window.location.hash = @redirectPath


module.exports = Pane
