crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicPagedTable = require './paged'

class MailingListPagedTable extends BasicPagedTable
  tag: 'mailinglist-paged-table'
  path: 'mailinglist'
  headers: [
    field('id', 'Name', 'id', 'id-display:name id-path:#mailinglist')
    field('mailchimp.listId', 'MailChimp List ID')
    field('id', 'Snippet', 'snippet')
    field('createdAt', 'Created On', 'date')
    field('updatedAt', 'Last Updated', 'ago')
  ]

MailingListPagedTable.register()
