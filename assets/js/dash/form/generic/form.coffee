_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
BasicFormView = require '../basic'

class GenericForm extends BasicFormView
  tag: 'form'
  redirectPath: ''
  path: ''

  # model that stores the last model queried
  resetModel: null

  inputConfigs:[]

  js: (opts)->
    #case sensitivity issues
    @id = id = opts.id

    if @id?
      @path += '/' + opts.id
    else
      @resetModel = JSON.parse JSON.stringify @model

    super

  reset: (event)->
    if event?
      event.preventDefault()

    @model = JSON.parse JSON.stringify @resetModel
    @initFormGroup.apply @
    @_reset(event)

    @obs.trigger Events.Form.Prefill, @model
    riot.update()

  _reset: (event)->

  _submit: (event)->
    p = super
    p.then ()=>
      if @id?
        @resetModel = JSON.parse JSON.stringify @model
      else
        @reset()

  loadData: (model)->
    @resetModel = JSON.parse JSON.stringify model

module.exports = GenericForm
