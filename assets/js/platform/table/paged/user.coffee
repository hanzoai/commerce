crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicPagedTable = require './paged'

class UserPagedTable extends BasicPagedTable
  tag: 'user-paged-table'
  path: 'user'
  headers: [
    field('id', 'Email', 'id', 'id-path:#user id-display:email')
    field('firstName', 'First Name')
    field('lastName', 'Last Name')
    field('createdAt', 'Created On', 'date')
    field('updatedAt', 'Last Updated', 'ago')
  ]

UserPagedTable.register()
