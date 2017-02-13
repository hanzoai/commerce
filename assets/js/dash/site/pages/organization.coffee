Page = require './page'

class Organization extends Page
  tag: 'page-organization'
  icon: 'fa fa-sitemap'
  name: 'Organization'
  html: require '../../templates/dash/site/pages/organization.html'

  collection: 'organization'

Organization.register()

module.exports = Organization
