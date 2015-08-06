_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
BasicForm = require '../basic'
FormView = crowdcontrol.view.form.FormView

Api = crowdcontrol.data.Api

class AdminForm extends FormView
  tag: 'admin-form'
  path: ''

  prefill: false

  html:     BasicForm.prototype.html
  events:   BasicForm.prototype.events
  _submit:  BasicForm.prototype._submit

  js: (opts)->
    super

    @api = api = Api.get 'platform'

    if @prefill
      api.get(@path).then((res)=>
        if res.status != 200
          throw new Error 'Form failed to load'

        @model = res.responseText

        @initFormGroup()

        @obs.trigger BasicForm.Events.Load, @model
        riot.update()
      ).catch (e)=>
        console.log(e.stack)

module.exports = AdminForm
