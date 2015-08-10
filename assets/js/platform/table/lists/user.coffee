crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicList = require './list'

class UserList extends BasicList
  tag: 'user-list'
  path: 'user'
  headers: [
    field('id', 'ID', 'id', 'id-path:#user id-display:email')
    field('firstName', 'First Name')
    field('lastName', 'Last Name')
    field('createdAt', 'Created On', 'date')
    field('updatedAt', 'Last Updated', 'ago')
  ]

UserList.register()
