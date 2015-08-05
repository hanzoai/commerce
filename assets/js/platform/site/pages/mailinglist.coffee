Page = require './page'

class MailingList extends Page
  tag: 'page-mailinglist'
  icon: 'fa fa-envelope'
  name: 'Mailing List'
  html: require '../../templates/backend/site/pages/mailinglist.html'

  collection: 'mailinglist'

MailingList.register()

module.exports = MailingList
