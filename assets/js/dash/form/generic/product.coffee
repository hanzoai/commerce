_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
Form = require './form'

Api = crowdcontrol.data.Api

class ProductForm extends Form
  tag: 'product-form'
  redirectPath: 'products'
  path: 'product'
  model:
    currency: 'usd'
    available: true

  inputConfigs: [
    input('id', '', 'static'),
    input('name', 'Product Name (Shirt)', 'required')
    input('slug', 'Product Slug (SHIRT-123)', 'required unique unique-api:product')
    input('sku', 'Product SKU/Barcode (12345678)', 'required')
    input('description', 'Describe this product', 'text')

    input('currency', '', 'currency-type-select'),
    input('listPrice', 'How much this should cost', 'money'),
    input('price', 'How much this costs right now', 'money'),
    input('estimatedDelivery', 'Estimated Delivery')

    input('size', '10cm x 10cm x 10cm'),
    input('weight', '1000', 'numeric'),

    input('available', '', 'switch'),

    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),
  ]

  loadData: (model)->
    super
    @inputConfigs[2].hints['unique-exception'] = model.slug

ProductForm.register()

module.exports = ProductForm
