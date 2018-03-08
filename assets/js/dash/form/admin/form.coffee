_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
BasicForm = require '../basic'
FormView = crowdcontrol.view.form.FormView

Api = crowdcontrol.data.Api

class AdminForm extends FormView
  tag: 'admin-form'
  path: ''
  processButtonText: 'Saving...'
  successButtonText: 'Saved'

  prefill: false

  html:     BasicForm.prototype.html
  events:   BasicForm.prototype.events
  reset: ()->
  _submit:  BasicForm.prototype._submit

  loadData: (model)->

  js: (opts)->
    super

    @api = api = Api.get 'dash'

    if @prefill
      api.get(@path).then((res)=>
        if res.status != 200
          throw new Error 'Form failed to load'

        @model = res.responseText

        @initFormGroup()

        @obs.trigger Events.Form.Prefill, @model
        riot.update()
      ).catch (e)->
        console.log(e.stack)

module.exports = AdminForm
