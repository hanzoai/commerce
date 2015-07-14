_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
BasicFormView = require '../basic'

class Form extends BasicFormView
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

    super

  reset: (event)->
    if event?
      event.preventDefault()

    @model = _.deepExtend {}, @resetModel
    @initFormGroup.apply @
    @_reset(event)
    riot.update()

  _reset: (event)->

  _submit: (event)->
    p = super
    p.then ()=>
      @resetModel = _.deepExtend {}, @model

  loadData: (model)->
    @resetModel = _.deepExtend {}, @model

module.exports = Form
