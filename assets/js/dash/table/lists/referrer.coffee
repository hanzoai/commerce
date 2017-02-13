crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicList = require './list'

m = crowdcontrol.utils.mediator

class ReferrerList extends BasicList
  tag: 'referrer-list'
  path: 'referrer'
  headers: [
    field('id', 'Referrer Token')
    field('createdAt', 'Created', 'date')
    # field('updatedAt', 'Last Updated', 'ago')
  ]

ReferrerList.register()
