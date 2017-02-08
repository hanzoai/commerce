_ = require 'underscore'

input = require '../input'
Form = require './form'

class OrganizationForm extends Form
  tag: 'organization-admin-form'
  path: 'organization'

  prefill: true

  inputConfigs: [
    input('name', '',  'static')
    input('fullName', 'Ex. Crowdstart',  'required')
    input('website', 'Ex. hanzo.io', '')
    input('emailWhitelist', 'Ex. your@email.com', 'text')
    input('googleAnalytics', '')
    input('facebookTag', '')
  ]

OrganizationForm.register()

module.exports = OrganizationForm
