_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
Form = require './form'

Api = crowdcontrol.data.Api

class MailingListForm extends Form
  tag: 'mailinglist-form'
  redirectPath: 'mailinglists'
  path: 'mailinglist'
  model:
    facebook:
      currency: 'usd'

  inputConfigs: [
    input('id', '', 'static'),
    input('name', 'Mailing List Name (Shirt)', 'required unique unique-api:mailinglist')
    input('thankyou.type', 'Choose what happens after form submit', 'mailinglist-thankyou-select required')
    input('thankyou.html', 'HTML ex. Thank You or <p style="font-weight:600">Thank You</p>\nUrl ex. /thankyou or www.yoursite.com/thankyou.html', 'text copy:thankyou.url')

    input('mailchimp.listId', 'ex. z1593c999e', 'required')
    input('mailchimp.apiKey', 'ex. myapikey-us2', 'required')

    input('mailchimp.doubleOptin', 'Double Optin?', 'switch')
    input('mailchimp.updateExisting', 'Update Existing?', 'switch')
    input('mailchimp.replaceInterests', 'Replace Interests?', 'switch')
    input('mailchimp.sendWelcome', 'Send Welcome?', 'switch')

    input('google.name', 'Event Name'),
    input('google.category', 'Event Category'),

    input('facebook.id', 'Event Id'),
    input('facebook.value', 'ex 0'),
    input('facebook.currency', 'Facebook Currency', 'currency-type-select'),

    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),
  ]

  loadData: (model)->
    super
    @inputConfigs[1].hints['unique-exception'] = model.name

MailingListForm.register()

module.exports = MailingListForm
