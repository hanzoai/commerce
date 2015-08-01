_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
Form = require './form'

Api = crowdcontrol.data.Api

class ProductForm extends Form
  tag: 'product-form'
  redirectPath: 'product'
  path: 'product'
  model:
    currency: 'usd'
    available: true

  inputConfigs: [
    input('name', 'Product Name (Shirt)', 'required')
    input('slug', 'Product Slug (SHIRT-123)', 'required unique unique-api:product')
    input('description', 'Describe this product', 'text')

    input('currency', '', 'currency-type-select'),
    input('listPrice', 'How much this should cost', 'money'),
    input('price', 'How much this costs right now', 'money'),

    input('size', '10cm x 10cm x 10cm'),
    input('weight', '1000', 'numeric'),

    input('available', '', 'switch'),
  ]

  loadData: (model)->
    super
    @inputConfigs[1].hints['unique-exception'] = model.slug

ProductForm.register()

module.exports = ProductForm
