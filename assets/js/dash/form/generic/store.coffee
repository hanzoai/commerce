_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
Form = require './form'

Api = crowdcontrol.data.Api

class StoreForm extends Form
  tag: 'store-form'
  redirectPath: 'stores'
  path: 'store'
  model:
    currency: 'usd'

  inputConfigs: [
    input('id', '', 'static'),
    input('name', 'Name', 'required')
    input('slug', 'Store Slug', 'required unique unique-api:store')
    input('currency', 'Store Currency', 'currency-type-select')

    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),
  ]

  loadData: (model)->
    super
    @inputConfigs[2].hints['unique-exception'] = model.slug

StoreForm.register()

module.exports = StoreForm
