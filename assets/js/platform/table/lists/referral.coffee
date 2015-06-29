crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicList = require './list'

m = crowdcontrol.utils.mediator

class ReferralList extends BasicList
  tag: 'referral-list'
  path: 'referral'
  headers: [
    field('userId', 'User Id', 'id', 'id-path:../user/')
    field('referrerId', 'Referral Token')
    field('createdAt', 'Referred On', 'date')
    # field('updatedAt', 'Last Updated', 'ago')
  ]

ReferralList.register()
