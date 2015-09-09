_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

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
  _submit:  BasicForm.prototype._submit

  js: (opts)->
    super

module.exports = Pane
