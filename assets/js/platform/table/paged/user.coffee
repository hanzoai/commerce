crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicPagedTable = require './paged'

class UserPagedTable extends BasicPagedTable
  tag: 'user-paged-table'
  path: 'user'
  headers: [
    field('id', 'ID', 'id', 'id-path:#user')
    field('email', 'Email')
    field('firstName', 'First Name')
    field('lastName', 'Last Name')
    field('createdAt', 'Created On', 'date')
    field('updatedAt', 'Last Updated', 'ago')
  ]

UserPagedTable.register()
