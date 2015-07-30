Page = require './page'

class MailingLists extends Page
  tag: 'page-mailinglists'
  icon: 'fa fa-envelope'
  name: 'MailingLists'
  html: require '../../templates/backend/site/pages/mailinglists.html'

  collection: 'dashboard'

MailingLists.register()

module.exports = MailingLists
