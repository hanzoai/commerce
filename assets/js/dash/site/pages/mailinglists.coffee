Page = require './page'

class MailingLists extends Page
  tag: 'page-mailinglists'
  icon: 'fa fa-envelope'
  name: 'Mailing Lists'
  html: require '../../templates/dash/site/pages/mailinglists.html'

  collection: 'mailinglists'

MailingLists.register()

module.exports = MailingLists
