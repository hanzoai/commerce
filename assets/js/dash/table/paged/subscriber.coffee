crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicPagedTable = require './paged'

class SubscriberPagedTable extends BasicPagedTable
  tag: 'subscriber-paged-table'
  path: 'subscriber'
  headers: [
    field('id', 'Email', 'id', 'id-path:#subscriber id-display:email')
    field('userId', 'User', 'id', 'id-path:#user')
    field('mailingListId', 'MailingList', 'id', 'id-path:#mailinglist')
    field('client.referer', 'Referrer', '', 'dontsort')
    field('unsubscribed', 'Unsubscribed', '')
    field('createdAt', 'Created', 'date')
  ]

SubscriberPagedTable.register()
