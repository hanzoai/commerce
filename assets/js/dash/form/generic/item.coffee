_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
BasicFormView = require '../basic'
Form = require './form'

Api = crowdcontrol.data.Api

class ItemForm extends Form
  tag: 'item-form'

  inputConfigs:[
    input('productId', '', 'product-type-select'),
    input('quantity', '', 'numeric'),
  ]

  js: ()->
    super

ItemForm.register()

