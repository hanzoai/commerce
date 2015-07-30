Page = require './page'

class Users extends Page
  tag: 'page-users'
  icon: 'fa fa-users'
  name: 'Users'
  html: require '../../templates/backend/site/pages/users.html'

  collection: 'users'

Users.register()

module.exports = Users
