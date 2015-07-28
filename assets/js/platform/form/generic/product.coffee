_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
m = crowdcontrol.utils.mediator

input = require '../input'
Form = require './form'

Api = crowdcontrol.data.Api

class ProductForm extends Form
  tag: 'product-form'
  redirectPath: 'products'
  path: 'product'

  inputConfigs:[
    input('id', '', 'static'),
    input('name', '', 'required'),
    input('slug', '', 'required'),
    input('description', '', 'text'),
    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),

    input('currency', '', 'currency-type-select'),
    input('listPrice', '', 'money'),
    input('price', '', 'money'),

    input('available', '', 'switch'),
    input('hidden', '', 'switch'),

    input('weight', '', 'weight'),
    input('dimensions', '(10cm x 10cm x 10cm)', 'dimensions'),
  ]

ProductForm.register()

module.exports = ProductForm
