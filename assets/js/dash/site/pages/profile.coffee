Page = require './page'

class Profile extends Page
  tag: 'page-profile'
  icon: 'glyphicon glyphicon-user'
  name: 'Profile'
  html: require '../../templates/backend/site/pages/profile.html'
  apiName: 'dash'

  collection: 'profile'

Profile.register()

module.exports = Profile
