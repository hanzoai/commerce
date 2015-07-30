Page = require './page'

class User extends Page
  tag: 'page-user'
  icon: 'fa fa-users'
  name: 'User'
  html: require '../../templates/backend/site/pages/user.html'

  collection: 'user'

User.register()

module.exports = User
